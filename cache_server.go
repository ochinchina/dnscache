package main

import (
	"fmt"
	"github.com/miekg/dns"
	"go.uber.org/zap"
	"strings"
	"sync"
	"time"
)

type CacheServer struct {
	listenAddr string
	servers    []string
	cache      *DNSCache
}

type DNSCacheItem struct {
	timeOut time.Time
	resp    *dns.Msg
}
type DNSCache struct {
	sync.Mutex
	cache map[string]*DNSCacheItem
}

func NewDNSCacheItem(resp *dns.Msg) *DNSCacheItem {
	timeout := time.Now().Add(time.Duration(resp.Answer[0].Header().Ttl) * time.Second)
	return &DNSCacheItem{timeOut: timeout, resp: resp}

}

func (dci *DNSCacheItem) isTimeout() bool {
	return time.Now().After(dci.timeOut)
}

func NewDNSCache() *DNSCache {
	return &DNSCache{cache: make(map[string]*DNSCacheItem)}
}

func (dc *DNSCache) findResponse(req *dns.Msg) (*dns.Msg, error) {
	key, err := dc.getKey(req)
	if err == nil {
		dc.Lock()
		defer dc.Unlock()
		if item, ok := dc.cache[key]; ok {
			if item.isTimeout() {
				delete(dc.cache, key)
			} else {
				zap.L().Debug("find response from cache", zap.String("req", fmt.Sprintf("%v", req ) ), zap.String("resp", fmt.Sprintf("%v", item.resp )))

				return item.resp, nil
			}

		}

	}
	return nil, fmt.Errorf("No response for %v", req)
}

func (dc *DNSCache) getKey(req *dns.Msg) (string, error) {
	if len(req.Question) == 1 {
		question := req.Question[0]

		if question.Qtype == dns.TypeAAAA || question.Qtype == dns.TypeA {
			return fmt.Sprintf("%s-%d", question.Name, question.Qtype), nil
		}
	}
	return "", fmt.Errorf("No key for %v", req)
}

func (dc *DNSCache) cacheResponse(req *dns.Msg, resp *dns.Msg) {
	if len(resp.Answer) > 0 {
		key, err := dc.getKey(req)
		if err == nil {
			zap.L().Debug("cache response", zap.String("req", fmt.Sprintf("%v", req ) ), zap.String("resp", fmt.Sprintf("%v", resp)))
			item := NewDNSCacheItem(resp)
			dc.Lock()
			defer dc.Unlock()

			dc.cache[key] = item

		}
	}
}

func parseListenAddr(listenAddr string) (proto string, addr string, err error) {
	if strings.HasPrefix(listenAddr, "udp:") {
		return "udp", listenAddr[4:], nil
	} else if strings.HasPrefix(listenAddr, "tcp:") {
		return "tcp", listenAddr[4:], nil
	} else {
		return "udp", listenAddr, nil
	}
}

func parseDNSServerAddr(dnsServerAddr string) (proto string, addr string, err error) {
	return parseListenAddr(dnsServerAddr)
}

func NewCacheServer(listenAddr string, servers []string) *CacheServer {
	return &CacheServer{listenAddr: listenAddr, servers: servers, cache: NewDNSCache()}
}

func (cs *CacheServer) start() error {
	proto, addr, err := parseListenAddr(cs.listenAddr)
	if err != nil {
		zap.L().Error("Fail to parse the listen address", zap.String("address", cs.listenAddr))
		return err
	}
	dns.HandleFunc(".", cs.processDNSMsg)

	if proto == "udp" {
		return cs.startUDPServer(addr)
	} else if proto == "tcp" {
		return cs.startTCPServer(addr)
	} else {
		zap.L().Error("Unsupported protocol", zap.String("protocol", proto))
		return fmt.Errorf("Unsupported protocol %s", proto)
	}
}

func (cs *CacheServer) startUDPServer(addr string) error {
	udpServer := &dns.Server{Addr: addr, Net: "udp"}
	zap.L().Info("start udp DNS server", zap.String("address", addr) )
	return udpServer.ListenAndServe()
}

func (cs *CacheServer) startTCPServer(addr string) error {
	tcpServer := &dns.Server{Addr: addr, Net: "tcp"}
 	zap.L().Info("start tcp DNS server", zap.String("address", addr) )
	return tcpServer.ListenAndServe()
}

func (cs *CacheServer) processDNSMsg(w dns.ResponseWriter, req *dns.Msg) {
	zap.L().Debug("process request", zap.String("request", fmt.Sprintf("%v", req) ) )
	resp, err := cs.cache.findResponse(req)
	if err == nil {
		resp.Id = req.Id
		w.WriteMsg(resp)
		return
	}
	for _, server := range cs.servers {
		proto, addr, err := parseDNSServerAddr(server)
		zap.L().Info("send request to DNS server", zap.String("server", server))
		c := &dns.Client{Net: proto}
		resp, _, err := c.Exchange(req, addr)
		if err == nil {
			cs.cache.cacheResponse(req, resp)
			zap.L().Info("succeed to get response", zap.String("response", fmt.Sprintf("%v", resp) ) )
			w.WriteMsg(resp)
			return
		}
	}
	zap.L().Error("fail to process request", zap.String("request", fmt.Sprintf("%v", req) ) )
}
