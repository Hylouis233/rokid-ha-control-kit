package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

type Config struct {
	Port              string
	HAURL             string
	HAToken           string
	RokidAuthAK       string
	EntityAliasesFile string
	AllowedEntities   map[string]struct{}
	AllowedServices   map[string]struct{}
	ConfirmToken      string
	AuditLogFile      string
}

type Server struct {
	cfg     Config
	client  *HAClient
	aliases []EntityAlias
}

type HAClient struct {
	baseURL string
	token   string
	http    *http.Client
}

type EntityAlias struct {
	EntityID string   `json:"entity_id"`
	Aliases  []string `json:"aliases"`
	Domain   string   `json:"domain,omitempty"`
}

type ServiceCallRequest struct {
	Domain       string                 `json:"domain"`
	Service      string                 `json:"service"`
	ServiceData  map[string]interface{} `json:"service_data"`
	ConfirmToken string                 `json:"confirm_token"`
}

type IntentRequest struct {
	Text         string `json:"text"`
	ConfirmToken string `json:"confirm_token"`
}

type RokidRequest struct {
	Text         string `json:"text"`
	Content      string `json:"content"`
	Query        string `json:"query"`
	Input        string `json:"input"`
	SessionID    string `json:"sessionId"`
	ConfirmToken string `json:"confirm_token"`
}

type AuditEvent struct {
	Time     string `json:"time"`
	Remote   string `json:"remote"`
	Action   string `json:"action"`
	Domain   string `json:"domain,omitempty"`
	Service  string `json:"service,omitempty"`
	EntityID string `json:"entity_id,omitempty"`
	Allowed  bool   `json:"allowed"`
	Reason   string `json:"reason,omitempty"`
}

var dangerousDomains = map[string]struct{}{
	"alarm_control_panel": {},
	"camera":              {},
	"lock":                {},
	"person":              {},
}

func main() {
	cfg := Config{
		Port:              env("PORT", "8080"),
		HAURL:             strings.TrimRight(os.Getenv("HA_URL"), "/"),
		HAToken:           os.Getenv("HA_TOKEN"),
		RokidAuthAK:       os.Getenv("ROKID_AUTH_AK"),
		EntityAliasesFile: env("ENTITY_ALIASES_FILE", "config/entity_aliases.example.json"),
		AllowedEntities:   parseSet(os.Getenv("ALLOWED_ENTITIES")),
		AllowedServices:   parseSet(os.Getenv("ALLOWED_SERVICES")),
		ConfirmToken:      os.Getenv("CONFIRM_TOKEN"),
		AuditLogFile:      os.Getenv("AUDIT_LOG_FILE"),
	}

	s := &Server{
		cfg: cfg,
		client: &HAClient{
			baseURL: cfg.HAURL,
			token:   cfg.HAToken,
			http:    &http.Client{Timeout: 20 * time.Second},
		},
	}
	s.aliases = loadAliases(cfg.EntityAliasesFile)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /entities", s.entities)
	mux.HandleFunc("POST /service-call", s.serviceCall)
	mux.HandleFunc("POST /intent", s.intent)
	mux.HandleFunc("POST /rokid/sse", s.rokidSSE)

	addr := ":" + cfg.Port
	log.Printf("rokid-ha-control-kit listening on %s", addr)
	if err := http.ListenAndServe(addr, logRequests(mux)); err != nil {
		log.Fatal(err)
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseSet(value string) map[string]struct{} {
	items := map[string]struct{}{}
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			items[item] = struct{}{}
		}
	}
	return items
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) entities(w http.ResponseWriter, r *http.Request) {
	body, status, err := s.client.get(r.Context(), "/api/states")
	if err != nil {
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeRawJSON(w, status, body)
}

func (s *Server) serviceCall(w http.ResponseWriter, r *http.Request) {
	var req ServiceCallRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if req.Domain == "" || req.Service == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "domain and service are required"})
		return
	}
	if err := s.authorizeCall(r, req, req.ConfirmToken); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}
	body, status, err := s.client.post(r.Context(), fmt.Sprintf("/api/services/%s/%s", req.Domain, req.Service), req.ServiceData)
	if err != nil {
		s.audit(r, req, false, err.Error())
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	s.audit(r, req, true, "")
	writeRawJSON(w, status, body)
}

func (s *Server) intent(w http.ResponseWriter, r *http.Request) {
	var req IntentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	result, status, err := s.handleIntent(r.Context(), r, req.Text, req.ConfirmToken)
	if err != nil {
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeRawJSON(w, status, result)
}

func (s *Server) rokidSSE(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeSSE(w, "error", map[string]string{"error": "unauthorized"})
		return
	}
	req, text, err := extractRokidRequest(r)
	if err != nil {
		writeSSE(w, "error", map[string]string{"error": err.Error()})
		return
	}
	writeSSE(w, "message", map[string]string{"content": "正在处理 Home Assistant 指令..."})
	result, _, err := s.handleIntent(r.Context(), r, text, req.ConfirmToken)
	if err != nil {
		writeSSE(w, "error", map[string]string{"error": err.Error()})
		return
	}
	writeSSE(w, "message", map[string]string{"content": summarizeResult(result)})
	writeSSE(w, "done", map[string]bool{"ok": true})
}

func (s *Server) handleIntent(ctx context.Context, r *http.Request, text, confirmToken string) ([]byte, int, error) {
	if strings.TrimSpace(text) == "" {
		return nil, http.StatusBadRequest, errors.New("text is required")
	}
	if call, ok := s.ruleIntent(text); ok {
		if err := s.authorizeCall(r, call, confirmToken); err != nil {
			return nil, http.StatusForbidden, err
		}
		body, status, err := s.client.post(ctx, fmt.Sprintf("/api/services/%s/%s", call.Domain, call.Service), call.ServiceData)
		if err != nil {
			s.audit(r, call, false, err.Error())
			return body, status, err
		}
		s.audit(r, call, true, "")
		return body, status, nil
	}
	return s.client.post(ctx, "/api/conversation/process", map[string]interface{}{"text": text, "language": "zh-cn"})
}

func (s *Server) ruleIntent(text string) (ServiceCallRequest, bool) {
	alias, ok := s.matchAlias(text)
	if !ok {
		return ServiceCallRequest{}, false
	}
	domain := alias.Domain
	if domain == "" {
		domain = strings.Split(alias.EntityID, ".")[0]
	}
	service := "turn_on"
	if containsAny(text, "关闭", "关掉", "关上") {
		service = "turn_off"
	} else if containsAny(text, "切换", "反转", "toggle") {
		service = "toggle"
	}
	data := map[string]interface{}{"entity_id": alias.EntityID}
	if domain == "light" && containsAny(text, "调暗", "暗一点") {
		data["brightness_pct"] = 35
		service = "turn_on"
	}
	if domain == "light" && containsAny(text, "调亮", "亮一点") {
		data["brightness_pct"] = 85
		service = "turn_on"
	}
	if domain == "climate" && containsAny(text, "温度", "空调") {
		if n, ok := findNumber(text); ok {
			service = "set_temperature"
			data["temperature"] = n
		}
	}
	return ServiceCallRequest{Domain: domain, Service: service, ServiceData: data}, true
}

func (s *Server) matchAlias(text string) (EntityAlias, bool) {
	for _, item := range s.aliases {
		if strings.Contains(text, item.EntityID) {
			return item, true
		}
		for _, alias := range item.Aliases {
			if alias != "" && strings.Contains(text, alias) {
				return item, true
			}
		}
	}
	return EntityAlias{}, false
}

func (s *Server) authorizeCall(r *http.Request, req ServiceCallRequest, confirmToken string) error {
	entityID := entityIDFromServiceData(req.ServiceData)
	if len(s.cfg.AllowedEntities) > 0 {
		if _, ok := s.cfg.AllowedEntities[entityID]; !ok {
			s.audit(r, req, false, "entity is not allowed")
			return errors.New("entity is not allowed")
		}
	}
	if len(s.cfg.AllowedServices) > 0 {
		serviceName := req.Domain + "." + req.Service
		if _, ok := s.cfg.AllowedServices[serviceName]; !ok {
			s.audit(r, req, false, "service is not allowed")
			return errors.New("service is not allowed")
		}
	}
	if isDangerousCall(req.Domain, req.Service) {
		if s.cfg.ConfirmToken == "" || confirmToken != s.cfg.ConfirmToken {
			s.audit(r, req, false, "dangerous operation requires confirmation")
			return errors.New("dangerous operation requires confirmation")
		}
	}
	return nil
}

func (s *Server) authorized(r *http.Request) bool {
	if s.cfg.RokidAuthAK == "" {
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		return host == "127.0.0.1" || host == "::1" || host == ""
	}
	if strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ") == s.cfg.RokidAuthAK {
		return true
	}
	if r.Header.Get("X-Auth-AK") == s.cfg.RokidAuthAK {
		return true
	}
	return false
}

func (c *HAClient) get(ctx context.Context, path string) ([]byte, int, error) {
	return c.do(ctx, http.MethodGet, path, nil)
}

func (c *HAClient) post(ctx context.Context, path string, payload interface{}) ([]byte, int, error) {
	return c.do(ctx, http.MethodPost, path, payload)
}

func (c *HAClient) do(ctx context.Context, method, path string, payload interface{}) ([]byte, int, error) {
	if c.baseURL == "" || c.token == "" {
		return nil, http.StatusServiceUnavailable, errors.New("HA_URL and HA_TOKEN must be configured")
	}
	var body io.Reader
	if payload != nil {
		buf, err := json.Marshal(payload)
		if err != nil {
			return nil, http.StatusBadRequest, err
		}
		body = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return nil, http.StatusBadGateway, err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, http.StatusBadGateway, err
	}
	if res.StatusCode >= 400 {
		return data, res.StatusCode, fmt.Errorf("home assistant returned %d: %s", res.StatusCode, string(data))
	}
	return data, res.StatusCode, nil
}

func loadAliases(path string) []EntityAlias {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var aliases []EntityAlias
	if err := json.Unmarshal(data, &aliases); err != nil {
		log.Printf("invalid aliases file: %v", err)
		return nil
	}
	return aliases
}

func extractText(r *http.Request) (string, error) {
	_, text, err := extractRokidRequest(r)
	return text, err
}

func extractRokidRequest(r *http.Request) (RokidRequest, string, error) {
	var req RokidRequest
	if err := decodeJSON(r, &req); err != nil {
		return req, "", err
	}
	for _, value := range []string{req.Text, req.Content, req.Query, req.Input} {
		if strings.TrimSpace(value) != "" {
			return req, strings.TrimSpace(value), nil
		}
	}
	return req, "", errors.New("text/content/query/input is required")
}

func decodeJSON(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	return dec.Decode(v)
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	data, _ := json.Marshal(payload)
	writeRawJSON(w, status, data)
}

func writeRawJSON(w http.ResponseWriter, status int, data []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func writeSSE(w http.ResponseWriter, event string, payload interface{}) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	data, _ := json.Marshal(payload)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func summarizeResult(data []byte) string {
	if len(data) == 0 || string(data) == "null" {
		return "操作已发送到 Home Assistant。"
	}
	if len(data) > 500 {
		return "操作已完成，Home Assistant 返回了较长结果。"
	}
	return "Home Assistant 返回：" + string(data)
}

func containsAny(text string, words ...string) bool {
	for _, word := range words {
		if strings.Contains(text, word) {
			return true
		}
	}
	return false
}

func findNumber(text string) (int, bool) {
	for i, r := range text {
		if r >= '0' && r <= '9' {
			j := i
			for j < len(text) && text[j] >= '0' && text[j] <= '9' {
				j++
			}
			var n int
			_, err := fmt.Sscanf(text[i:j], "%d", &n)
			return n, err == nil
		}
	}
	return 0, false
}

func isDangerousCall(domain, service string) bool {
	_, blocked := dangerousDomains[domain]
	return blocked
}

func entityIDFromServiceData(data map[string]interface{}) string {
	if data == nil {
		return ""
	}
	value, ok := data["entity_id"]
	if !ok {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case []string:
		if len(v) > 0 {
			return v[0]
		}
	case []interface{}:
		if len(v) > 0 {
			if s, ok := v[0].(string); ok {
				return s
			}
		}
	}
	return ""
}

func (s *Server) audit(r *http.Request, req ServiceCallRequest, allowed bool, reason string) {
	if s.cfg.AuditLogFile == "" {
		return
	}
	event := AuditEvent{
		Time:     time.Now().Format(time.RFC3339),
		Remote:   r.RemoteAddr,
		Action:   "service_call",
		Domain:   req.Domain,
		Service:  req.Service,
		EntityID: entityIDFromServiceData(req.ServiceData),
		Allowed:  allowed,
		Reason:   reason,
	}
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("audit marshal failed: %v", err)
		return
	}
	f, err := os.OpenFile(s.cfg.AuditLogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		log.Printf("audit open failed: %v", err)
		return
	}
	defer f.Close()
	_, _ = f.Write(append(data, '\n'))
}
