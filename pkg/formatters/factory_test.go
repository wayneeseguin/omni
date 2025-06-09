package formatters

import (
	"errors"
	"fmt"
	"testing"

	"github.com/wayneeseguin/omni/pkg/types"
)

func TestFactory_NewFactory(t *testing.T) {
	f := NewFactory()

	if f == nil {
		t.Fatal("NewFactory() returned nil")
	}

	if f.formatters == nil {
		t.Fatal("formatters map not initialized")
	}

	// Check default formatters are registered
	formatters := f.ListFormatters()
	found := make(map[string]bool)
	for _, name := range formatters {
		found[name] = true
	}

	if !found["text"] {
		t.Error("text formatter not registered by default")
	}
	if !found["json"] {
		t.Error("json formatter not registered by default")
	}
}

func TestFactory_Register(t *testing.T) {
	f := NewFactory()

	tests := []struct {
		name        string
		formatName  string
		constructor FormatterConstructor
		wantErr     bool
	}{
		{
			name:       "valid registration",
			formatName: "custom",
			constructor: func() (types.Formatter, error) {
				return &mockFormatter{}, nil
			},
			wantErr: false,
		},
		{
			name:        "empty name",
			formatName:  "",
			constructor: func() (types.Formatter, error) { return nil, nil },
			wantErr:     true,
		},
		{
			name:        "nil constructor",
			formatName:  "nil-constructor",
			constructor: nil,
			wantErr:     true,
		},
		{
			name:       "override existing",
			formatName: "text",
			constructor: func() (types.Formatter, error) {
				return &mockFormatter{}, nil
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := f.Register(tt.formatName, tt.constructor)
			if (err != nil) != tt.wantErr {
				t.Errorf("Register() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFactory_CreateFormatter(t *testing.T) {
	f := NewFactory()

	// Register a test formatter
	testErr := errors.New("test error")
	_ = f.Register("test-formatter", func() (types.Formatter, error) {
		return &mockFormatter{}, nil
	})
	_ = f.Register("error-formatter", func() (types.Formatter, error) {
		return nil, testErr
	})

	tests := []struct {
		name      string
		formatter string
		wantErr   bool
		checkErr  func(error) bool
	}{
		{
			name:      "create text formatter",
			formatter: "text",
			wantErr:   false,
		},
		{
			name:      "create json formatter",
			formatter: "json",
			wantErr:   false,
		},
		{
			name:      "create test formatter",
			formatter: "test-formatter",
			wantErr:   false,
		},
		{
			name:      "non-existent formatter",
			formatter: "does-not-exist",
			wantErr:   true,
		},
		{
			name:      "formatter with error",
			formatter: "error-formatter",
			wantErr:   true,
			checkErr: func(err error) bool {
				return errors.Is(err, testErr)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter, err := f.CreateFormatter(tt.formatter)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateFormatter() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && formatter == nil {
				t.Error("CreateFormatter() returned nil formatter without error")
			}

			if tt.checkErr != nil && err != nil && !tt.checkErr(err) {
				t.Errorf("CreateFormatter() error = %v, does not match expected", err)
			}
		})
	}
}

func TestFactory_CreateFormatterByType(t *testing.T) {
	f := NewFactory()

	tests := []struct {
		name       string
		formatType int
		wantErr    bool
		checkType  func(types.Formatter) bool
	}{
		{
			name:       "text format type",
			formatType: FormatText,
			wantErr:    false,
			checkType: func(f types.Formatter) bool {
				_, ok := f.(*TextFormatter)
				return ok
			},
		},
		{
			name:       "json format type",
			formatType: FormatJSON,
			wantErr:    false,
			checkType: func(f types.Formatter) bool {
				_, ok := f.(*JSONFormatter)
				return ok
			},
		},
		{
			name:       "custom format type",
			formatType: FormatCustom,
			wantErr:    true,
		},
		{
			name:       "unknown format type",
			formatType: 999,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter, err := f.CreateFormatterByType(tt.formatType)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateFormatterByType() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && formatter == nil {
				t.Error("CreateFormatterByType() returned nil formatter without error")
			}

			if tt.checkType != nil && formatter != nil && !tt.checkType(formatter) {
				t.Errorf("CreateFormatterByType() returned wrong formatter type")
			}
		})
	}
}

func TestFactory_ListFormatters(t *testing.T) {
	f := NewFactory()

	// Register additional formatters
	_ = f.Register("custom1", func() (types.Formatter, error) { return nil, nil })
	_ = f.Register("custom2", func() (types.Formatter, error) { return nil, nil })

	formatters := f.ListFormatters()

	// Should have at least the default formatters plus custom ones
	if len(formatters) < 4 {
		t.Errorf("expected at least 4 formatters, got %d", len(formatters))
	}

	// Check specific formatters exist
	found := make(map[string]bool)
	for _, name := range formatters {
		found[name] = true
	}

	expected := []string{"text", "json", "custom1", "custom2"}
	for _, name := range expected {
		if !found[name] {
			t.Errorf("expected formatter %s in list", name)
		}
	}
}

func TestFactory_Concurrent(t *testing.T) {
	f := NewFactory()

	// Test concurrent registration and creation
	done := make(chan bool)

	// Register formatters concurrently
	go func() {
		for i := 0; i < 100; i++ {
			name := fmt.Sprintf("formatter%d", i)
			_ = f.Register(name, func() (types.Formatter, error) {
				return &mockFormatter{}, nil
			})
		}
		done <- true
	}()

	// Create formatters concurrently
	go func() {
		for i := 0; i < 100; i++ {
			_, _ = f.CreateFormatter("text")
			_, _ = f.CreateFormatter("json")
		}
		done <- true
	}()

	// List formatters concurrently
	go func() {
		for i := 0; i < 100; i++ {
			_ = f.ListFormatters()
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		<-done
	}
}

func TestDefaultFactory(t *testing.T) {
	// Test global functions use default factory
	err := Register("global-test", func() (types.Formatter, error) {
		return &mockFormatter{}, nil
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	formatter, err := CreateFormatter("global-test")
	if err != nil {
		t.Fatalf("CreateFormatter() error = %v", err)
	}
	if formatter == nil {
		t.Error("CreateFormatter() returned nil")
	}

	// Test CreateFormatterByType
	formatter, err = CreateFormatterByType(FormatJSON)
	if err != nil {
		t.Fatalf("CreateFormatterByType() error = %v", err)
	}
	if _, ok := formatter.(*JSONFormatter); !ok {
		t.Error("CreateFormatterByType() returned wrong type")
	}
}

// mockFormatter is a test formatter
type mockFormatter struct{}

func (m *mockFormatter) Format(msg types.LogMessage) ([]byte, error) {
	return []byte("mock"), nil
}
