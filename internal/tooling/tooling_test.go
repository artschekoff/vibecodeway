package tooling

import (
	"context"
	"testing"
)

func TestRegistryRegisterListGet(t *testing.T) {
	r := &Registry{}
	r.Register(Tool{Name: "b", Description: "second"})
	r.Register(Tool{Name: "a", Description: "first", Write: true})

	list := r.List()
	if len(list) != 2 || list[0].Name != "b" || list[1].Name != "a" {
		t.Fatalf("List must preserve registration order, got %+v", list)
	}
	got, ok := r.Get("a")
	if !ok || !got.Write {
		t.Fatalf("Get(a) = %+v, %v; want Write tool", got, ok)
	}
	if _, ok := r.Get("missing"); ok {
		t.Fatalf("Get(missing) should be false")
	}
}

func TestRegistryCall(t *testing.T) {
	r := &Registry{}
	r.Register(Tool{Name: "echo", Handler: func(_ context.Context, args map[string]any, _ Ctx) (any, error) {
		return args["v"], nil
	}})

	out, err := r.Call(context.Background(), "echo", map[string]any{"v": "hi"}, Ctx{})
	if err != nil || out != "hi" {
		t.Fatalf("Call(echo) = %v, %v; want hi, nil", out, err)
	}
	if _, err := r.Call(context.Background(), "nope", nil, Ctx{}); err == nil {
		t.Fatalf("Call(nope) should error on unknown tool")
	}
}
