package core

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/0x6d61/guri/internal/scheme"
	"github.com/0x6d61/guri/internal/vm"
)

// These inputs keep unit tests independent of the machine clock/environment.
// This is private runtime state, not an embedding interface.
type systemContext struct {
	commandLine []string
	lookup      func(string) (string, bool)
	environ     func() []string
	now         func() time.Time
	since       func(time.Time) time.Duration
	epoch       time.Time
}

// SetCommandLine supplies the invocation selected by the command-line frontend.
// It is internal to the interpreter; the root Go API is unchanged.
func (r *Runtime) SetCommandLine(args ...string) error {
	if r.running {
		return vm.ErrReentrant
	}
	if len(args) == 0 {
		return fmt.Errorf("command-line requires a command name")
	}
	for _, arg := range args {
		if _, err := scheme.NewString(arg); err != nil {
			return err
		}
	}
	r.system.commandLine = append([]string(nil), args...)
	return nil
}
func immutableSystemString(s string) (*scheme.String, error) {
	v, err := scheme.NewString(s)
	if err != nil {
		return nil, err
	}
	scheme.FreezeLiteral(v)
	return v, nil
}
func environmentEntry(s string) (string, string, error) {
	start := 0
	if strings.HasPrefix(s, "=") {
		start = 1
	} // Windows' hidden per-drive variables.
	index := strings.IndexByte(s[start:], '=')
	if index < 0 {
		return "", "", fmt.Errorf("invalid OS environment entry")
	}
	index += start
	return s[:index], s[index+1:], nil
}

// R7RS permits approximating TAI by UTC plus a suitable constant. USNO's
// tai-utc.dat records TAI-UTC = 37 seconds from 2017-01-01; update this constant
// when IERS announces a new offset. No runtime network request is made.
// https://maia.usno.navy.mil/ser7/tai-utc.dat
const taiMinusUTC = 37

func (r *Runtime) installSystem() {
	args := append([]string(nil), os.Args...)
	if len(args) == 0 {
		args = []string{"guri"}
	}
	r.system = &systemContext{commandLine: args, lookup: os.LookupEnv, environ: os.Environ, now: time.Now, since: time.Since, epoch: time.Now()}
	e := r.bootstrap
	for _, name := range []string{"command-line", "get-environment-variable", "get-environment-variables", "current-second", "current-jiffy", "jiffies-per-second"} {
		required := 0
		if name == "get-environment-variable" {
			required = 1
		}
		e.Bind(name, vm.Native(name, required, required, func(c *vm.Context, a []scheme.Value) (vm.Action, error) {
			result, err := r.systemValue(name, a)
			if err != nil {
				return pairFailure(c, name, a, err)
			}
			return vm.Return(result), nil
		}))
	}
	e.primitives = append([]*binding(nil), e.globals...)
}
func (r *Runtime) systemValue(name string, a []scheme.Value) (scheme.Value, error) {
	switch name {
	case "command-line":
		values := make([]scheme.Value, 0, len(r.system.commandLine))
		for _, arg := range r.system.commandLine {
			v, err := immutableSystemString(arg)
			if err != nil {
				return nil, err
			}
			values = append(values, v)
		}
		return scheme.List(values...), nil
	case "get-environment-variable":
		key, ok := a[0].(*scheme.String)
		if !ok {
			return nil, scheme.ErrType
		}
		value, found := r.system.lookup(key.Text())
		if !found {
			return scheme.False, nil
		}
		return immutableSystemString(value)
	case "get-environment-variables":
		var values []scheme.Value
		for _, entry := range r.system.environ() {
			key, value, err := environmentEntry(entry)
			if err != nil {
				return nil, err
			}
			k, err := immutableSystemString(key)
			if err != nil {
				return nil, err
			}
			v, err := immutableSystemString(value)
			if err != nil {
				return nil, err
			}
			values = append(values, scheme.Cons(k, v))
		}
		result := scheme.List(values...)
		scheme.FreezeLiteral(result)
		return result, nil
	case "current-second":
		now := r.system.now()
		return scheme.NewFloat(float64(now.Unix()) + float64(now.Nanosecond())/1e9 + taiMinusUTC), nil
	case "current-jiffy":
		return scheme.NewInteger(r.system.since(r.system.epoch).Nanoseconds()), nil
	case "jiffies-per-second":
		return scheme.NewInteger(1_000_000_000), nil
	}
	return nil, scheme.ErrType
}
