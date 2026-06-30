package main

import "testing"

func TestSendEditResultDataIncludesPersistenceStatus(t *testing.T) {
	data := sendEditResultData("120@g.us", "edit-id", "target-id", false, "missing target")
	if data["sent"] != true || data["to"] != "120@g.us" || data["id"] != "edit-id" || data["target_id"] != "target-id" {
		t.Fatalf("data = %+v", data)
	}
	if data["persisted"] != false || data["persist_error"] != "missing target" {
		t.Fatalf("data = %+v, want persistence status", data)
	}
}
