package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/miekg/dns"
)

const (
	maxSnapshotBytes = 16 << 20
	maxZones         = 20_000
	maxRecords       = 50_000
)

type record struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	Value    string `json:"value"`
	TTL      uint32 `json:"ttl"`
	Priority uint16 `json:"priority"`
	Proxied  bool   `json:"proxied"`
}

type zoneInput struct {
	Name    string   `json:"name"`
	Records []record `json:"records"`
}

type zonesResponse struct {
	Zones []zoneInput `json:"zones"`
}

type zone struct {
	name        string
	answers     map[string]map[uint16][]dns.RR
	soa         dns.RR
	nameServers []dns.RR
}

type snapshot struct {
	zones     map[string]*zone
	serial    uint32
	digest    string
	lastSync  time.Time
	zoneCount int
}

type zoneStore struct {
	current atomic.Pointer[snapshot]
}

type serverConfig struct {
	dnsAddr      string
	httpAddr     string
	streamURL    string
	streamKey    string
	pollInterval time.Duration
	proxyIPv4    net.IP
	proxyIPv6    net.IP
	nameServers  []string
	soaMName     string
	soaRName     string
	soaTTL       uint32
	negativeTTL  uint32
}

func main() {
	config, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	store := &zoneStore{}
	store.current.Store(&snapshot{zones: map[string]*zone{}})
	go pollStream(context.Background(), store, config)

	dnsMux := dns.NewServeMux()
	dnsMux.HandleFunc(".", store.serveDNS(config))
	udpServer := &dns.Server{Addr: config.dnsAddr, Net: "udp", Handler: dnsMux, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second}
	tcpServer := &dns.Server{Addr: config.dnsAddr, Net: "tcp", Handler: dnsMux, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErrors := make(chan error, 3)
	go func() { serverErrors <- udpServer.ListenAndServe() }()
	go func() { serverErrors <- tcpServer.ListenAndServe() }()

	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeHealth(w, store.current.Load(), true)
	})
	healthMux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		current := store.current.Load()
		writeHealth(w, current, current.lastSync.IsZero() == false)
	})
	httpServer := &http.Server{
		Addr:              config.httpAddr,
		Handler:           healthMux,
		ReadHeaderTimeout: 3 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	go func() { serverErrors <- httpServer.ListenAndServe() }()

	log.Printf("NetGoat authoritative DNS listening on %s (UDP/TCP); health on %s", config.dnsAddr, config.httpAddr)
	select {
	case <-ctx.Done():
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server stopped: %v", err)
			stop()
		}
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdown)
	_ = udpServer.Shutdown()
	_ = tcpServer.Shutdown()
}

func loadConfig() (serverConfig, error) {
	streamKey := strings.TrimSpace(os.Getenv("STREAM_DNS_KEY"))
	if streamKey == "" {
		return serverConfig{}, errors.New("STREAM_DNS_KEY is required")
	}
	pollSeconds := envInt("DNS_POLL_SECONDS", 5, 1, 3600)
	ttl := uint32(envInt("DNS_SOA_TTL", 300, 60, 86400))
	negativeTTL := uint32(envInt("DNS_NEGATIVE_TTL", 60, 1, 86400))

	nameServers := splitNames(os.Getenv("DNS_NAMESERVERS"))
	if len(nameServers) == 0 {
		return serverConfig{}, errors.New("DNS_NAMESERVERS must contain the public DNS hostnames for this deployment")
	}
	for index, name := range nameServers {
		if !validFQDN(name) {
			return serverConfig{}, fmt.Errorf("DNS_NAMESERVERS contains an invalid name at position %d", index+1)
		}
		nameServers[index] = dns.Fqdn(strings.ToLower(strings.TrimSuffix(name, ".")))
	}
	soaMName := strings.TrimSpace(os.Getenv("DNS_SOA_MNAME"))
	if soaMName == "" {
		soaMName = nameServers[0]
	}
	soaRName := strings.TrimSpace(os.Getenv("DNS_SOA_RNAME"))
	if soaRName == "" {
		soaRName = "hostmaster.netgoat.xyz."
	}
	if !validFQDN(soaMName) || !validFQDN(soaRName) {
		return serverConfig{}, errors.New("DNS_SOA_MNAME and DNS_SOA_RNAME must be valid DNS names")
	}

	proxyIPv4, err := parseProxyIP("DNS_PROXY_IPV4", true)
	if err != nil {
		return serverConfig{}, err
	}
	proxyIPv6, err := parseProxyIP("DNS_PROXY_IPV6", false)
	if err != nil {
		return serverConfig{}, err
	}

	return serverConfig{
		dnsAddr:      envString("DNS_ADDR", ":1053"),
		httpAddr:     envString("DNS_HTTP_ADDR", ":8053"),
		streamURL:    strings.TrimRight(envString("STREAM_URL", "http://streamer:8787"), "/"),
		streamKey:    streamKey,
		pollInterval: time.Duration(pollSeconds) * time.Second,
		proxyIPv4:    proxyIPv4,
		proxyIPv6:    proxyIPv6,
		nameServers:  nameServers,
		soaMName:     dns.Fqdn(strings.ToLower(strings.TrimSuffix(soaMName, "."))),
		soaRName:     dns.Fqdn(strings.ToLower(strings.TrimSuffix(soaRName, "."))),
		soaTTL:       ttl,
		negativeTTL:  negativeTTL,
	}, nil
}

func parseProxyIP(variable string, ipv4 bool) (net.IP, error) {
	value := strings.TrimSpace(os.Getenv(variable))
	if value == "" {
		return nil, nil
	}
	ip := net.ParseIP(value)
	if ip == nil || (ipv4 && ip.To4() == nil) || (!ipv4 && (ip.To4() != nil || ip.To16() == nil)) {
		family := "IPv6"
		if ipv4 {
			family = "IPv4"
		}
		return nil, fmt.Errorf("%s must be a valid %s address", variable, family)
	}
	if ipv4 {
		return ip.To4(), nil
	}
	return ip.To16(), nil
}

func envString(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envInt(name string, fallback, minimum, maximum int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
	if err != nil || value < minimum || value > maximum {
		return fallback
	}
	return value
}

func splitNames(value string) []string {
	var names []string
	for _, item := range strings.Split(value, ",") {
		if name := strings.TrimSpace(item); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func pollStream(ctx context.Context, store *zoneStore, config serverConfig) {
	client := &http.Client{Timeout: 8 * time.Second}
	ticker := time.NewTicker(config.pollInterval)
	defer ticker.Stop()
	for {
		if err := store.refresh(ctx, client, config); err != nil {
			log.Printf("stream snapshot refresh failed; retaining last valid zones: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (store *zoneStore) refresh(ctx context.Context, client *http.Client, config serverConfig) error {
	requestCtx, cancel := context.WithTimeout(ctx, client.Timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, config.streamURL+"/dns/zones", nil)
	if err != nil {
		return fmt.Errorf("create stream request: %w", err)
	}
	request.Header.Set("X-DNS-API-Key", config.streamKey)
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("request stream snapshot: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return fmt.Errorf("stream server returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxSnapshotBytes+1))
	if err != nil {
		return fmt.Errorf("read stream snapshot: %w", err)
	}
	if len(body) > maxSnapshotBytes {
		return fmt.Errorf("stream snapshot exceeds %d bytes", maxSnapshotBytes)
	}
	var input zonesResponse
	if err := json.Unmarshal(body, &input); err != nil {
		return fmt.Errorf("decode stream snapshot: %w", err)
	}
	if len(input.Zones) > maxZones {
		return fmt.Errorf("stream snapshot has too many zones")
	}
	next, err := buildSnapshot(input.Zones, config, store.current.Load())
	if err != nil {
		return err
	}
	store.current.Store(next)
	return nil
}

func buildSnapshot(inputs []zoneInput, config serverConfig, previous *snapshot) (*snapshot, error) {
	zoneNames := make([]string, 0, len(inputs))
	inputByName := make(map[string]zoneInput, len(inputs))
	count := 0
	for _, input := range inputs {
		name := normalizeZoneName(input.Name)
		if name == "" {
			continue
		}
		if _, exists := inputByName[name]; exists {
			return nil, fmt.Errorf("stream snapshot has duplicate zone %q", name)
		}
		count += len(input.Records)
		if count > maxRecords {
			return nil, fmt.Errorf("stream snapshot has too many records")
		}
		inputByName[name] = input
		zoneNames = append(zoneNames, name)
	}
	sort.Strings(zoneNames)

	zones := make(map[string]*zone, len(inputByName))
	for _, name := range zoneNames {
		input := inputByName[name]
		z := &zone{name: name, answers: make(map[string]map[uint16][]dns.RR)}
		for _, item := range input.Records {
			for _, rr := range recordRRs(name, item, config) {
				owner := strings.ToLower(rr.Header().Name)
				if z.answers[owner] == nil {
					z.answers[owner] = make(map[uint16][]dns.RR)
				}
				z.answers[owner][rr.Header().Rrtype] = append(z.answers[owner][rr.Header().Rrtype], rr)
			}
		}
		// CNAME cannot coexist with other data at the same owner name.
		for owner, rrsets := range z.answers {
			if len(rrsets) > 1 {
				delete(rrsets, dns.TypeCNAME)
			}
			if len(rrsets) == 0 {
				delete(z.answers, owner)
			}
		}

		apex := z.answers[name]
		z.nameServers = append([]dns.RR(nil), apex[dns.TypeNS]...)
		if len(z.nameServers) == 0 {
			for _, nameServer := range config.nameServers {
				z.nameServers = append(z.nameServers, &dns.NS{
					Hdr: dns.RR_Header{Name: name, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: config.soaTTL},
					Ns:  nameServer,
				})
			}
			if z.answers[name] == nil {
				z.answers[name] = make(map[uint16][]dns.RR)
			}
			z.answers[name][dns.TypeNS] = append([]dns.RR(nil), z.nameServers...)
		}
		serial := uint32(time.Now().UTC().Unix())
		soaTTL := config.soaTTL
		if soaTTL > config.negativeTTL {
			soaTTL = config.negativeTTL
		}
		z.soa = &dns.SOA{
			Hdr:     dns.RR_Header{Name: name, Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: soaTTL},
			Ns:      config.soaMName,
			Mbox:    config.soaRName,
			Serial:  serial,
			Refresh: 3600,
			Retry:   600,
			Expire:  86400,
			Minttl:  config.negativeTTL,
		}
		zones[name] = z
	}

	canonical, err := json.Marshal(inputs)
	if err != nil {
		return nil, fmt.Errorf("serialize DNS snapshot: %w", err)
	}
	digestBytes := sha256.Sum256(canonical)
	digest := hex.EncodeToString(digestBytes[:])
	serial := uint32(time.Now().UTC().Unix())
	if previous != nil {
		if digest == previous.digest {
			serial = previous.serial
		} else if serial <= previous.serial {
			serial = previous.serial + 1
		}
	}
	for _, z := range zones {
		z.soa.(*dns.SOA).Serial = serial
	}
	return &snapshot{zones: zones, serial: serial, digest: digest, lastSync: time.Now().UTC(), zoneCount: len(zones)}, nil
}

func normalizeZoneName(value string) string {
	name := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(value), "."))
	if !validFQDN(name) {
		return ""
	}
	return dns.Fqdn(name)
}

func validFQDN(value string) bool {
	name := strings.TrimSuffix(strings.TrimSpace(value), ".")
	if name == "" || strings.ContainsAny(name, " \t\r\n") {
		return false
	}
	_, ok := dns.IsDomainName(dns.Fqdn(name))
	return ok
}

func recordOwner(zoneName, name string) string {
	if name == "@" || name == "" {
		return zoneName
	}
	owner := dns.Fqdn(strings.ToLower(name + "." + strings.TrimSuffix(zoneName, ".")))
	if dns.IsSubDomain(zoneName, owner) {
		return owner
	}
	return ""
}

func recordRRs(zoneName string, item record, config serverConfig) []dns.RR {
	owner := recordOwner(zoneName, strings.TrimSpace(item.Name))
	if owner == "" || item.TTL < 1 {
		return nil
	}
	typeName := strings.ToUpper(strings.TrimSpace(item.Type))
	rdata := strings.TrimSpace(item.Value)
	if rdata == "" || strings.ContainsAny(rdata, "\r\n\x00") {
		return nil
	}

	if item.Proxied {
		switch typeName {
		case "A":
			if config.proxyIPv4 == nil {
				return nil
			}
			return []dns.RR{&dns.A{Hdr: rrHeader(owner, dns.TypeA, item.TTL), A: config.proxyIPv4}}
		case "AAAA":
			if config.proxyIPv6 == nil {
				return nil
			}
			return []dns.RR{&dns.AAAA{Hdr: rrHeader(owner, dns.TypeAAAA, item.TTL), AAAA: config.proxyIPv6}}
		case "CNAME":
			var records []dns.RR
			if config.proxyIPv4 != nil {
				records = append(records, &dns.A{Hdr: rrHeader(owner, dns.TypeA, item.TTL), A: config.proxyIPv4})
			}
			if config.proxyIPv6 != nil {
				records = append(records, &dns.AAAA{Hdr: rrHeader(owner, dns.TypeAAAA, item.TTL), AAAA: config.proxyIPv6})
			}
			return records
		}
	}

	switch typeName {
	case "A":
		ip := net.ParseIP(rdata)
		if ip == nil || ip.To4() == nil {
			return nil
		}
		return []dns.RR{&dns.A{Hdr: rrHeader(owner, dns.TypeA, item.TTL), A: ip.To4()}}
	case "AAAA":
		ip := net.ParseIP(rdata)
		if ip == nil || ip.To4() != nil || ip.To16() == nil {
			return nil
		}
		return []dns.RR{&dns.AAAA{Hdr: rrHeader(owner, dns.TypeAAAA, item.TTL), AAAA: ip.To16()}}
	case "CNAME", "NS":
		if !validFQDN(rdata) {
			return nil
		}
		target := dns.Fqdn(strings.ToLower(strings.TrimSuffix(rdata, ".")))
		if typeName == "CNAME" {
			if owner == zoneName {
				return nil
			}
			return []dns.RR{&dns.CNAME{Hdr: rrHeader(owner, dns.TypeCNAME, item.TTL), Target: target}}
		}
		return []dns.RR{&dns.NS{Hdr: rrHeader(owner, dns.TypeNS, item.TTL), Ns: target}}
	case "MX":
		if !validFQDN(rdata) {
			return nil
		}
		return []dns.RR{&dns.MX{Hdr: rrHeader(owner, dns.TypeMX, item.TTL), Preference: item.Priority, Mx: dns.Fqdn(strings.ToLower(strings.TrimSuffix(rdata, ".")))}}
	case "TXT":
		return []dns.RR{&dns.TXT{Hdr: rrHeader(owner, dns.TypeTXT, item.TTL), Txt: splitTXT(rdata)}}
	case "SRV":
		parts := strings.Fields(rdata)
		if len(parts) != 4 || !validFQDN(parts[3]) {
			return nil
		}
		priority, err1 := strconv.ParseUint(parts[0], 10, 16)
		weight, err2 := strconv.ParseUint(parts[1], 10, 16)
		port, err3 := strconv.ParseUint(parts[2], 10, 16)
		if err1 != nil || err2 != nil || err3 != nil {
			return nil
		}
		return []dns.RR{&dns.SRV{Hdr: rrHeader(owner, dns.TypeSRV, item.TTL), Priority: uint16(priority), Weight: uint16(weight), Port: uint16(port), Target: dns.Fqdn(strings.ToLower(strings.TrimSuffix(parts[3], ".")))}}
	case "CAA":
		rr, err := dns.NewRR(fmt.Sprintf("%s %d IN CAA %s", owner, item.TTL, rdata))
		if err != nil {
			return nil
		}
		return []dns.RR{rr}
	default:
		return nil
	}
}

func rrHeader(owner string, rrType uint16, ttl uint32) dns.RR_Header {
	return dns.RR_Header{Name: owner, Rrtype: rrType, Class: dns.ClassINET, Ttl: ttl}
}

func splitTXT(value string) []string {
	bytes := []byte(value)
	parts := make([]string, 0, (len(bytes)+254)/255)
	for len(bytes) > 255 {
		parts = append(parts, string(bytes[:255]))
		bytes = bytes[255:]
	}
	return append(parts, string(bytes))
}

func (store *zoneStore) serveDNS(config serverConfig) dns.HandlerFunc {
	return func(w dns.ResponseWriter, request *dns.Msg) {
		response := new(dns.Msg)
		response.SetReply(request)
		response.RecursionAvailable = false
		if request.Opcode != dns.OpcodeQuery || len(request.Question) != 1 {
			response.SetRcode(request, dns.RcodeFormatError)
			_ = w.WriteMsg(response)
			return
		}
		question := request.Question[0]
		if question.Qclass != dns.ClassINET {
			response.SetRcode(request, dns.RcodeNotImplemented)
			_ = w.WriteMsg(response)
			return
		}
		if question.Qtype == dns.TypeAXFR || question.Qtype == dns.TypeIXFR || question.Qtype == dns.TypeANY {
			response.SetRcode(request, dns.RcodeRefused)
			_ = w.WriteMsg(response)
			return
		}

		current := store.current.Load()
		name := strings.ToLower(dns.Fqdn(question.Name))
		selected := ""
		for candidate := range current.zones {
			if name == candidate || dns.IsSubDomain(candidate, name) {
				if len(candidate) > len(selected) {
					selected = candidate
				}
			}
		}
		if selected == "" {
			response.SetRcode(request, dns.RcodeRefused)
			_ = w.WriteMsg(response)
			return
		}
		zone := current.zones[selected]
		response.Authoritative = true
		if question.Qtype == dns.TypeSOA && name == selected {
			response.Answer = []dns.RR{dns.Copy(zone.soa)}
			_ = w.WriteMsg(response)
			return
		}

		types := zone.answers[name]
		if question.Qtype == dns.TypeNS && name == selected && len(types[dns.TypeNS]) > 0 {
			response.Answer = cloneRRs(types[dns.TypeNS])
		} else if question.Qtype == dns.TypeCNAME && len(types[dns.TypeCNAME]) > 0 {
			response.Answer = cloneRRs(types[dns.TypeCNAME])
		} else if cnames := types[dns.TypeCNAME]; len(cnames) > 0 && question.Qtype != dns.TypeCNAME {
			response.Answer = cloneRRs(cnames)
		} else if answers := types[question.Qtype]; len(answers) > 0 {
			response.Answer = cloneRRs(answers)
		}
		if len(response.Answer) == 0 {
			if len(types) == 0 && name != selected {
				response.Rcode = dns.RcodeNameError
			}
			response.Ns = []dns.RR{dns.Copy(zone.soa)}
		}
		_ = w.WriteMsg(response)
	}
}

func cloneRRs(records []dns.RR) []dns.RR {
	result := make([]dns.RR, len(records))
	for index, record := range records {
		result[index] = dns.Copy(record)
	}
	return result
}

func writeHealth(w http.ResponseWriter, current *snapshot, ready bool) {
	status := http.StatusOK
	state := "ok"
	if !ready {
		status = http.StatusServiceUnavailable
		state = "waiting-for-stream"
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":    state,
		"service":   "netgoat-dns",
		"zones":     current.zoneCount,
		"last_sync": current.lastSync,
	})
}
