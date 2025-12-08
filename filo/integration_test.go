package filo

import (
	"testing"
)

// TestDoString executes a simple Filo script that assigns a global variable
// and then verifies that the variable was correctly set.
func TestDoString(t *testing.T) {
	f := New()
	defer f.Close()

	// Execute Filo script setting global variable 'x' to 42.
	if err := f.DoString("(set x 42)"); err != nil {
		t.Fatalf("DoString error: %v", err)
	}

	// Retrieve 'x' and verify its value.
	x := f.MustGetInt("x")
	if x != 42 {
		t.Fatalf("Expected x = 42, got %d", x)
	}
}

// TestSetGlobalInt verifies that an integer is correctly set as a global variable.
func TestSetGlobalInt(t *testing.T) {
	f := New()
	defer f.Close()

	f.SetGlobal("num", 100)
	num := f.MustGetInt("num")
	if num != 100 {
		t.Fatalf("Expected num = 100, got %d", num)
	}
}

// TestSetGlobalString verifies that a string is correctly set as a global variable.
func TestSetGlobalString(t *testing.T) {
	f := New()
	defer f.Close()

	f.SetGlobal("greeting", "hello")
	greeting := f.MustGetString("greeting")
	if greeting != "hello" {
		t.Fatalf("Expected greeting = 'hello', got %s", greeting)
	}
}

// TestSetGlobalBool verifies that a bool is correctly set as a global variable.
func TestSetGlobalBool(t *testing.T) {
	f := New()
	defer f.Close()

	f.SetGlobal("enabled", true)
	enabled := f.MustGetBool("enabled")
	if !enabled {
		t.Fatalf("Expected enabled = true, got false")
	}

	f.SetGlobal("disabled", false)
	disabled := f.MustGetBool("disabled")
	if disabled {
		t.Fatalf("Expected disabled = false, got true")
	}
}

// TestSetGlobalTable verifies that a []string is correctly set as a Filo list global.
func TestSetGlobalTable(t *testing.T) {
	f := New()
	defer f.Close()

	expected := []string{"one", "two", "three"}
	f.SetGlobal("list", expected)

	list := f.MustGetTable("list")
	if len(list) != len(expected) {
		t.Fatalf("Expected list length %d, got %d", len(expected), len(list))
	}
	for i, v := range expected {
		if list[i] != v {
			t.Fatalf("Expected list[%d] = %s, got %s", i, v, list[i])
		}
	}
}

// TestSetGlobalMap verifies that a map[string]string is correctly set as a Filo global.
func TestSetGlobalMap(t *testing.T) {
	f := New()
	defer f.Close()

	m := make(map[string]string)
	m["one"] = "uno"
	m["two"] = "dos"
	m["three"] = "tres"
	f.SetGlobal("map", m)

	mapTable := f.MustGetMap("map")
	if len(mapTable) != len(m) {
		t.Fatalf("Expected map length %d, got %d", len(m), len(mapTable))
	}
	for k, v := range m {
		if mapTable[k] != v {
			t.Fatalf("Expected map[%s] = %s, got %s", k, v, mapTable[k])
		}
	}
}

// TestScriptWithGlobals verifies that globals set before script execution are available in the script.
func TestScriptWithGlobals(t *testing.T) {
	f := New()
	defer f.Close()

	f.SetGlobal("base", 10)
	f.SetGlobal("multiplier", 5)

	script := "(set result (* base multiplier))"
	if err := f.DoString(script); err != nil {
		t.Fatalf("DoString error: %v", err)
	}

	result := f.MustGetInt("result")
	if result != 50 {
		t.Fatalf("Expected result = 50, got %d", result)
	}
}

// TestScriptWithConditional verifies that conditional logic works in config scripts.
func TestScriptWithConditional(t *testing.T) {
	f := New()
	defer f.Close()

	f.SetGlobal("env", "prod")

	script := `(if (= env "prod")
		(set port 443)
		(set port 8080))`
	if err := f.DoString(script); err != nil {
		t.Fatalf("DoString error: %v", err)
	}

	port := f.MustGetInt("port")
	if port != 443 {
		t.Fatalf("Expected port = 443, got %d", port)
	}
}

// TestMultipleStatements verifies that multiple set statements can be executed.
func TestMultipleStatements(t *testing.T) {
	f := New()
	defer f.Close()

	script := `(let ()
		(set host "localhost")
		(set port 3210)
		(set enabled #t))`
	if err := f.DoString(script); err != nil {
		t.Fatalf("DoString error: %v", err)
	}

	host := f.MustGetString("host")
	if host != "localhost" {
		t.Fatalf("Expected host = 'localhost', got %s", host)
	}

	port := f.MustGetInt("port")
	if port != 3210 {
		t.Fatalf("Expected port = 3210, got %d", port)
	}

	enabled := f.MustGetBool("enabled")
	if !enabled {
		t.Fatalf("Expected enabled = true, got false")
	}
}
