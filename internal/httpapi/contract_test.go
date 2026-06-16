package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"pawit-vetcare/internal/domain"
)

func TestHospitalPortalReadResponseContracts(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())

	tests := []struct {
		name        string
		path        string
		role        domain.Role
		topKeys     []string
		listField   string
		itemKeys    []string
		objectField string
		objectKeys  []string
	}{
		{
			name:        "current user",
			path:        "/api/v1/me",
			topKeys:     []string{"user", "clinic"},
			objectField: "user",
			objectKeys:  []string{"id", "role", "tenantId"},
		},
		{
			name:      "navigation",
			path:      "/api/v1/navigation",
			topKeys:   []string{"sections"},
			listField: "sections",
			itemKeys:  []string{"label", "items"},
		},
		{
			name:      "locations",
			path:      "/api/v1/locations",
			topKeys:   []string{"items"},
			listField: "items",
			itemKeys:  []string{"id", "name", "timezone", "status"},
		},
		{
			name:      "dashboard summary",
			path:      "/api/v1/dashboard/summary",
			topKeys:   []string{"metrics"},
			listField: "metrics",
			itemKeys:  []string{"label", "value"},
		},
		{
			name:      "appointments",
			path:      "/api/v1/appointments",
			topKeys:   []string{"items"},
			listField: "items",
			itemKeys:  []string{"id", "petName", "ownerName", "type", "status"},
		},
		{
			name:      "calendar",
			path:      "/api/v1/calendar",
			topKeys:   []string{"date", "statusCounts", "items"},
			listField: "items",
			itemKeys:  []string{"id", "petName", "status"},
		},
		{
			name:      "queue",
			path:      "/api/v1/queue",
			topKeys:   []string{"items"},
			listField: "items",
			itemKeys:  []string{"id", "petName", "ownerName", "status", "waitMins"},
		},
		{
			name:      "pets",
			path:      "/api/v1/pets",
			topKeys:   []string{"items"},
			listField: "items",
			itemKeys:  []string{"id", "petName", "ownerName", "species", "documentsCount"},
		},
		{
			name:      "pet documents",
			path:      "/api/v1/pets/pet_001/documents",
			topKeys:   []string{"items"},
			listField: "items",
			itemKeys:  []string{"id", "petId", "title", "documentType", "objectPath", "contentType", "sizeBytes", "status", "createdAt"},
		},
		{
			name:      "prescriptions",
			path:      "/api/v1/prescriptions",
			role:      domain.RoleVeterinarian,
			topKeys:   []string{"items"},
			listField: "items",
			itemKeys:  []string{"id", "petName", "status", "medicationNames", "sharedWithPetParent"},
		},
		{
			name:      "prescription templates",
			path:      "/api/v1/prescription-templates",
			topKeys:   []string{"items"},
			listField: "items",
			itemKeys:  []string{"id", "name", "medications", "instructions"},
		},
		{
			name:      "clinical notes",
			path:      "/api/v1/clinical-notes",
			topKeys:   []string{"items"},
			listField: "items",
			itemKeys:  []string{"id", "petName", "subject", "status", "sharedWithPetParent"},
		},
		{
			name:      "lab tests",
			path:      "/api/v1/lab-tests",
			topKeys:   []string{"items"},
			listField: "items",
			itemKeys:  []string{"id", "petName", "testType", "status", "sharedWithPetParent"},
		},
		{
			name:      "billing",
			path:      "/api/v1/billing",
			topKeys:   []string{"metrics", "invoices"},
			listField: "metrics",
			itemKeys:  []string{"label", "value"},
		},
		{
			name:      "analytics",
			path:      "/api/v1/analytics",
			topKeys:   []string{"metrics", "speciesDistribution", "appointmentStatus", "revenueTrend", "commonDiagnoses"},
			listField: "metrics",
			itemKeys:  []string{"label", "value"},
		},
		{
			name:      "feedback",
			path:      "/api/v1/feedback",
			topKeys:   []string{"metrics", "distribution", "items"},
			listField: "metrics",
			itemKeys:  []string{"label", "value"},
		},
		{
			name:      "doctors",
			path:      "/api/v1/doctors",
			topKeys:   []string{"items"},
			listField: "items",
			itemKeys:  []string{"id", "name", "role", "email", "status"},
		},
		{
			name:      "staff",
			path:      "/api/v1/staff",
			topKeys:   []string{"items"},
			listField: "items",
			itemKeys:  []string{"id", "name", "role", "email", "status"},
		},
		{
			name:      "audit logs",
			path:      "/api/v1/audit-logs",
			topKeys:   []string{"items"},
			listField: "items",
			itemKeys:  []string{"id", "actorUserId", "actorRole", "action", "resourceType", "createdAt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			role := tt.role
			if role == "" {
				role = domain.RoleClinicAdmin
			}
			payload := getJSONContractPayload(t, server, tt.path, role)

			requireKeys(t, payload, tt.topKeys)
			if tt.objectField != "" {
				requireObjectKeys(t, payload, tt.objectField, tt.objectKeys)
			}
			if tt.listField != "" {
				requireFirstItemKeys(t, payload, tt.listField, tt.itemKeys)
			}
		})
	}
}

func TestProductPolicyResponseContract(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	payload := getJSONContractPayload(t, server, "/api/v1/product-spec", domain.RoleClinicAdmin)

	requireKeys(t, payload, []string{
		"productName",
		"supportedSpecies",
		"supportedAppointmentTypes",
		"supportedAppointmentStatuses",
		"supportedLabStatuses",
		"paymentProvider",
		"currency",
		"telemedicine",
		"labs",
		"cancellationPolicy",
	})
	requireObjectKeys(t, payload, "telemedicine", []string{"mode", "builtInVideoRoomsEnabled"})
	requireObjectKeys(t, payload, "labs", []string{"internalLabsEnabled", "externalLabsEnabled", "externalIntegrationMode"})
	requireObjectKeys(t, payload, "cancellationPolicy", []string{"defaultCutoffHours", "tenantConfigurable", "staffOverrideAllowed"})
}

func TestRolePoliciesResponseContract(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	payload := getJSONContractPayload(t, server, "/api/v1/role-policies", domain.RoleClinicAdmin)

	requireKeys(t, payload, []string{"items"})
	requireFirstItemKeys(t, payload, "items", []string{"role", "description", "permissions"})
}

func TestAPIContractDocsMatchRegisteredRoutes(t *testing.T) {
	registered := registeredAPIRoutesFromSource(t)
	documented := documentedAPIRoutes(t)

	requireStringSetEqual(t, registered, documented)
}

func getJSONContractPayload(t *testing.T, server http.Handler, path string, role domain.Role) map[string]any {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(role))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("%s expected status %d, got %d: %s", path, http.StatusOK, response.Code, response.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode %s response: %v", path, err)
	}
	return payload
}

func requireKeys(t *testing.T, payload map[string]any, keys []string) {
	t.Helper()

	for _, key := range keys {
		if _, ok := payload[key]; !ok {
			t.Fatalf("expected top-level key %q in %#v", key, payload)
		}
	}
}

func requireObjectKeys(t *testing.T, payload map[string]any, field string, keys []string) {
	t.Helper()

	value, ok := payload[field]
	if !ok {
		t.Fatalf("expected object field %q", field)
	}
	object, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected %q to be an object, got %T", field, value)
	}
	requireKeys(t, object, keys)
}

func requireFirstItemKeys(t *testing.T, payload map[string]any, field string, keys []string) {
	t.Helper()

	value, ok := payload[field]
	if !ok {
		t.Fatalf("expected list field %q", field)
	}
	items, ok := value.([]any)
	if !ok {
		t.Fatalf("expected %q to be a list, got %T", field, value)
	}
	if len(items) == 0 {
		t.Fatalf("expected %q to include at least one item", field)
	}
	first, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first %q item to be an object, got %T", field, items[0])
	}
	requireKeys(t, first, keys)
}

func registeredAPIRoutesFromSource(t *testing.T) []string {
	t.Helper()

	content, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatalf("read server route source: %v", err)
	}

	matches := regexp.MustCompile(`s\.mux\.HandleFunc\("([A-Z]+) (/api/v1/[^"]+)"`).FindAllStringSubmatch(string(content), -1)
	routes := make([]string, 0, len(matches))
	for _, match := range matches {
		routes = append(routes, match[1]+" "+match[2])
	}
	sort.Strings(routes)
	return routes
}

func documentedAPIRoutes(t *testing.T) []string {
	t.Helper()

	content, err := os.ReadFile("../../docs/api-contract.md")
	if err != nil {
		t.Fatalf("read API contract docs: %v", err)
	}

	matches := regexp.MustCompile(`(?m)^\| `+"`"+`(GET|POST)`+"`"+` \| `+"`"+`([^`+"`"+`]+)`+"`"+` \|`).FindAllStringSubmatch(string(content), -1)
	routes := make([]string, 0, len(matches))
	for _, match := range matches {
		path := match[2]
		if !strings.HasPrefix(path, "/api/v1/") {
			path = "/api/v1" + path
		}
		routes = append(routes, match[1]+" "+path)
	}
	sort.Strings(routes)
	return routes
}

func requireStringSetEqual(t *testing.T, actual []string, expected []string) {
	t.Helper()

	actualOnly, expectedOnly := diffSortedStrings(actual, expected)
	if len(actualOnly) > 0 || len(expectedOnly) > 0 {
		t.Fatalf("API contract docs drifted from registered routes\nregistered only: %v\ndocumented only: %v", actualOnly, expectedOnly)
	}
}

func diffSortedStrings(actual []string, expected []string) ([]string, []string) {
	actualSet := make(map[string]struct{}, len(actual))
	for _, item := range actual {
		actualSet[item] = struct{}{}
	}
	expectedSet := make(map[string]struct{}, len(expected))
	for _, item := range expected {
		expectedSet[item] = struct{}{}
	}

	var actualOnly []string
	for item := range actualSet {
		if _, ok := expectedSet[item]; !ok {
			actualOnly = append(actualOnly, item)
		}
	}
	var expectedOnly []string
	for item := range expectedSet {
		if _, ok := actualSet[item]; !ok {
			expectedOnly = append(expectedOnly, item)
		}
	}
	sort.Strings(actualOnly)
	sort.Strings(expectedOnly)
	return actualOnly, expectedOnly
}
