package polling

import (
	"github.com/robertkrimen/otto"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

// SetupOttoVM registers universal built-in functions and historical values on Otto VM,
// providing 100% compatibility with TWSNMP FK script execution.
func SetupOttoVM(pe *datastore.PollingEnt, vm *otto.Otto, fields map[string]interface{}) {
	if vm == nil {
		return
	}
	if fields == nil {
		fields = make(map[string]interface{})
	}

	// Register all current fields directly into the Otto VM
	for k, v := range fields {
		_ = vm.Set(k, v)
	}

	// setResult(name, value): Sets custom extracted metric or calculated value
	_ = vm.Set("setResult", func(call otto.FunctionCall) otto.Value {
		if call.Argument(0).IsString() {
			n := call.Argument(0).String()
			arg1 := call.Argument(1)
			if arg1.IsNumber() {
				if v, err := arg1.ToFloat(); err == nil {
					fields[n] = v
					if pe.Result != nil {
						pe.Result[n] = v
					}
					_ = vm.Set(n, v)
				}
			} else if arg1.IsString() {
				s := arg1.String()
				fields[n] = s
				if pe.Result != nil {
					pe.Result[n] = s
				}
				_ = vm.Set(n, s)
			} else if arg1.IsBoolean() {
				var val float64
				if b, err := arg1.ToBoolean(); err == nil && b {
					val = 1
				} else {
					val = 0
				}
				fields[n] = val
				if pe.Result != nil {
					pe.Result[n] = val
				}
				_ = vm.Set(n, val)
			}
		}
		return otto.Value{}
	})

	// getResult(name): Retrieves a current or previously set metric value
	_ = vm.Set("getResult", func(call otto.FunctionCall) otto.Value {
		if call.Argument(0).IsString() {
			k := call.Argument(0).String()
			if v, ok := fields[k]; ok {
				if ov, err := otto.ToValue(v); err == nil {
					return ov
				}
			}
			if pe.Result != nil {
				if v, ok := pe.Result[k]; ok {
					if ov, err := otto.ToValue(v); err == nil {
						return ov
					}
				}
			}
		}
		return otto.UndefinedValue()
	})

	// setLevel(level): Dynamically overrides the polling status level ("normal", "repair", "warn", "low", "high", "info", "off")
	_ = vm.Set("setLevel", func(call otto.FunctionCall) otto.Value {
		if call.Argument(0).IsString() {
			level := call.Argument(0).String()
			fields["_level"] = level
			if pe.Result != nil {
				pe.Result["_level"] = level
			}
		}
		return otto.Value{}
	})

	// saveReport(type, object): Saves reporting data (e.g., env monitor, user login)
	_ = vm.Set("saveReport", func(call otto.FunctionCall) otto.Value {
		if len(call.ArgumentList) != 2 ||
			!call.Argument(0).IsString() ||
			!call.Argument(1).IsObject() {
			return otto.FalseValue()
		}
		if o, err := call.Argument(1).Export(); err == nil {
			if m, ok := o.(map[string]interface{}); ok {
				for k, v := range m {
					fields["report_"+k] = v
				}
				return otto.TrueValue()
			}
		}
		return otto.FalseValue()
	})

	// Historical values: inject previous Result keys as <key>_last
	if pe.Result != nil && len(pe.Result) > 0 {
		for k, v := range pe.Result {
			if k != "error" && k != "_level" {
				_ = vm.Set(k+"_last", v)
			}
		}
	}

	// Interval (pe.PollInt in seconds)
	_ = vm.Set("interval", float64(pe.PollInt))
	_ = vm.Set("iterval", float64(pe.PollInt)) // FK compatibility for legacy typo
}
