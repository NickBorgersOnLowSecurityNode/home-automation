package testutil

import "time"

// ServiceCall records a service call for testing/verification
type ServiceCall struct {
	Timestamp   time.Time
	Domain      string
	Service     string
	ServiceData map[string]interface{}
}

// FilterServiceCalls filters service calls by domain and service
func FilterServiceCalls(calls []ServiceCall, domain, service string) []ServiceCall {
	var filtered []ServiceCall
	for _, call := range calls {
		if call.Domain == domain && call.Service == service {
			filtered = append(filtered, call)
		}
	}
	return filtered
}

// FindServiceCallWithData finds a service call with matching data key/value
func FindServiceCallWithData(calls []ServiceCall, domain, service, dataKey string, dataValue interface{}) *ServiceCall {
	for i := len(calls) - 1; i >= 0; i-- {
		call := calls[i]
		if call.Domain == domain && call.Service == service {
			if val, ok := call.ServiceData[dataKey]; ok && val == dataValue {
				return &call
			}
		}
	}
	return nil
}

// FindServiceCallWithEntityID finds a service call for a specific entity
func FindServiceCallWithEntityID(calls []ServiceCall, domain, service, entityID string) *ServiceCall {
	return FindServiceCallWithData(calls, domain, service, "entity_id", entityID)
}
