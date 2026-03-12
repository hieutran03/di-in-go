// Package container provides a minimal reflection-based DI container that
// mirrors the public API of go.uber.org/dig without the external dependency.
//
// It is used exclusively by cmd/06_runtime_container to demonstrate the
// runtime DI container pattern.  Production code should use go.uber.org/dig
// or go.uber.org/fx directly.
package container

import (
	"context"
	"fmt"
	"reflect"
)

// ── Provider ──────────────────────────────────────────────────────────────────

type provider struct {
	constructor reflect.Value // the constructor function
	retType     reflect.Type  // return type[0] — the type this provider produces
}

// ── Container ─────────────────────────────────────────────────────────────────

// Container resolves and caches dependencies by type using reflection.
// All registered types are treated as Singletons — each constructor is
// called at most once.
type Container struct {
	providers []provider
	cache     map[reflect.Type]reflect.Value
	onStart   []func(context.Context) error
	onStop    []func(context.Context) error
}

func New() *Container {
	return &Container{cache: make(map[reflect.Type]reflect.Value)}
}

// Provide registers a constructor function.
// The function may return (T) or (T, error).
// Registration order does not matter — the graph is resolved lazily.
func (c *Container) Provide(constructor any) {
	t := reflect.TypeOf(constructor)
	if t.Kind() != reflect.Func {
		panic(fmt.Sprintf("container.Provide: expected func, got %s", t.Kind()))
	}
	if t.NumOut() == 0 {
		panic("container.Provide: constructor must return at least one value")
	}
	c.providers = append(c.providers, provider{
		constructor: reflect.ValueOf(constructor),
		retType:     t.Out(0),
	})
}

// Invoke resolves all parameters of fn and calls it.
// fn must be a function; its return value (if error) is returned to the caller.
func (c *Container) Invoke(fn any) error {
	t := reflect.TypeOf(fn)
	if t.Kind() != reflect.Func {
		return fmt.Errorf("container.Invoke: expected func, got %s", t.Kind())
	}
	args, err := c.resolveArgs(t)
	if err != nil {
		return err
	}
	results := reflect.ValueOf(fn).Call(args)
	if len(results) > 0 {
		if errVal := results[len(results)-1]; !errVal.IsNil() {
			return errVal.Interface().(error)
		}
	}
	return nil
}

// OnStart registers a hook called during Start, after all providers are wired.
func (c *Container) OnStart(fn func(context.Context) error) { c.onStart = append(c.onStart, fn) }

// OnStop registers a hook called during Stop for graceful shutdown.
func (c *Container) OnStop(fn func(context.Context) error) { c.onStop = append(c.onStop, fn) }

// Start runs all OnStart hooks in registration order.
func (c *Container) Start(ctx context.Context) error {
	for _, fn := range c.onStart {
		if err := fn(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Stop runs all OnStop hooks in reverse registration order.
func (c *Container) Stop(ctx context.Context) error {
	for i := len(c.onStop) - 1; i >= 0; i-- {
		if err := c.onStop[i](ctx); err != nil {
			return err
		}
	}
	return nil
}

// ── internals ─────────────────────────────────────────────────────────────────

func (c *Container) resolve(t reflect.Type) (reflect.Value, error) {
	// Return cached value (Singleton behaviour).
	if v, ok := c.cache[t]; ok {
		return v, nil
	}
	// Find a provider for this type.
	for _, p := range c.providers {
		if p.retType == t || (t.Kind() == reflect.Interface && p.retType.Implements(t)) {
			args, err := c.resolveArgs(p.constructor.Type())
			if err != nil {
				return reflect.Value{}, err
			}
			results := p.constructor.Call(args)
			val := results[0]
			// Check for construction error.
			if len(results) == 2 && !results[1].IsNil() {
				return reflect.Value{}, results[1].Interface().(error)
			}
			c.cache[t] = val
			return val, nil
		}
	}
	return reflect.Value{}, fmt.Errorf("container: no provider registered for %s", t)
}

func (c *Container) resolveArgs(fnType reflect.Type) ([]reflect.Value, error) {
	args := make([]reflect.Value, fnType.NumIn())
	for i := range args {
		v, err := c.resolve(fnType.In(i))
		if err != nil {
			return nil, err
		}
		args[i] = v
	}
	return args, nil
}
