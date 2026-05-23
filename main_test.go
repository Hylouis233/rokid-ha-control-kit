package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestRuleIntentBrightness(t *testing.T) {
	server := &Server{aliases: []EntityAlias{{EntityID: "light.living_room", Domain: "light", Aliases: []string{"客厅灯"}}}}
	call, ok := server.ruleIntent("把客厅灯调亮一点")
	if !ok {
		t.Fatal("expected alias match")
	}
	if call.Service != "turn_on" {
		t.Fatalf("unexpected service: %s", call.Service)
	}
	if call.ServiceData["brightness_pct"] != 85 {
		t.Fatalf("unexpected brightness: %#v", call.ServiceData)
	}
}

func TestAuthorizedRejectsMissingToken(t *testing.T) {
	server := &Server{cfg: Config{RokidAuthAK: "secret"}}
	req := httptest.NewRequest(http.MethodPost, "/rokid/sse", bytes.NewReader([]byte(`{"text":"开灯"}`)))
	req.RemoteAddr = "127.0.0.1:12345"
	if server.authorized(req) {
		t.Fatal("expected unauthorized")
	}
	req.Header.Set("X-Auth-AK", "secret")
	if !server.authorized(req) {
		t.Fatal("expected authorized with X-Auth-AK")
	}
}

func TestExtractTextAcceptsContentField(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/rokid/sse", bytes.NewReader([]byte(`{"content":"打开客厅灯"}`)))
	text, err := extractText(req)
	if err != nil {
		t.Fatal(err)
	}
	if text != "打开客厅灯" {
		t.Fatalf("unexpected text: %q", text)
	}
}

func TestSSEHealth(t *testing.T) {
	server := &Server{cfg: Config{}}
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	server.health(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rr.Code)
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte(`"status":"ok"`)) {
		t.Fatalf("unexpected body: %s", rr.Body.Bytes())
	}
}

func TestServiceCallPayloadEncoding(t *testing.T) {
	data, err := json.Marshal(ServiceCallRequest{Domain: "light", Service: "turn_on", ServiceData: map[string]interface{}{"entity_id": "light.living_room"}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"domain":"light"`)) {
		t.Fatalf("unexpected payload: %s", data)
	}
}

func TestAuthorizeCallRejectsEntityOutsideAllowlist(t *testing.T) {
	server := &Server{cfg: Config{AllowedEntities: map[string]struct{}{"light.living_room": {}}}}
	req := httptest.NewRequest(http.MethodPost, "/service-call", nil)
	call := ServiceCallRequest{Domain: "light", Service: "turn_on", ServiceData: map[string]interface{}{"entity_id": "light.kitchen"}}

	err := server.authorizeCall(req, call, "")
	if err == nil || err.Error() != "entity is not allowed" {
		t.Fatalf("expected entity allowlist rejection, got %v", err)
	}
}

func TestAuthorizeCallRejectsServiceOutsideAllowlist(t *testing.T) {
	server := &Server{cfg: Config{AllowedServices: map[string]struct{}{"light.turn_on": {}}}}
	req := httptest.NewRequest(http.MethodPost, "/service-call", nil)
	call := ServiceCallRequest{Domain: "light", Service: "turn_off", ServiceData: map[string]interface{}{"entity_id": "light.living_room"}}

	err := server.authorizeCall(req, call, "")
	if err == nil || err.Error() != "service is not allowed" {
		t.Fatalf("expected service allowlist rejection, got %v", err)
	}
}

func TestAuthorizeCallRequiresConfirmTokenForDangerousDomain(t *testing.T) {
	server := &Server{cfg: Config{ConfirmToken: "confirm"}}
	req := httptest.NewRequest(http.MethodPost, "/service-call", nil)
	call := ServiceCallRequest{Domain: "lock", Service: "unlock", ServiceData: map[string]interface{}{"entity_id": "lock.front_door"}}

	if err := server.authorizeCall(req, call, "wrong"); err == nil || err.Error() != "dangerous operation requires confirmation" {
		t.Fatalf("expected dangerous operation rejection, got %v", err)
	}
	if err := server.authorizeCall(req, call, "confirm"); err != nil {
		t.Fatalf("expected confirmed dangerous operation to pass, got %v", err)
	}
}

func TestAuditWritesJSONLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	server := &Server{cfg: Config{AuditLogFile: path}}
	req := httptest.NewRequest(http.MethodPost, "/service-call", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	call := ServiceCallRequest{Domain: "light", Service: "turn_on", ServiceData: map[string]interface{}{"entity_id": "light.living_room"}}

	server.audit(req, call, true, "")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"entity_id":"light.living_room"`)) || !bytes.HasSuffix(data, []byte("\n")) {
		t.Fatalf("unexpected audit log: %s", data)
	}
}
