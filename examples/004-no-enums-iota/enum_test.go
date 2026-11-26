package main

import "testing"

func TestOrderStatusString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in        OrderStatus
		want      string
		wantValid bool
	}{
		{Unknown, "Unknown", true},
		{Pending, "Pending", true},
		{Processing, "Processing", true},
		{Shipped, "Shipped", true},
		{Delivered, "Delivered", true},
		// Out of range on both sides: the slide's String would panic here.
		{Delivered + 1, "OrderStatus(5)", false},
		{OrderStatus(42), "OrderStatus(42)", false},
		{OrderStatus(-1), "OrderStatus(-1)", false},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			if got := tt.in.String(); got != tt.want {
				t.Errorf("OrderStatus(%d).String() = %q, want %q", int(tt.in), got, tt.want)
			}
			if got := tt.in.IsValid(); got != tt.wantValid {
				t.Errorf("OrderStatus(%d).IsValid() = %t, want %t", int(tt.in), got, tt.wantValid)
			}
		})
	}
}

func TestHandleOrderRejectsInvalid(t *testing.T) {
	t.Parallel()
	for _, s := range []OrderStatus{-1, Delivered + 1, 99} {
		want := "rejected: invalid status " + s.String()
		if got := HandleOrder(s); got != want {
			t.Errorf("HandleOrder(%d) = %q, want %q", int(s), got, want)
		}
	}
}

func TestRoleHas(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		perms Role
		flag  Role
		want  bool
	}{
		{"single bit set", Admin | Viewer, Admin, true},
		{"single bit unset", Admin | Viewer, Editor, false},
		{"all of several bits", Admin | Editor | Viewer, Admin | Editor, true},
		{"only some of several bits", Admin, Admin | Editor, false},
		{"after bit clear", (Admin | Viewer) &^ Admin, Admin, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.perms.Has(tt.flag); got != tt.want {
				t.Errorf("%04b.Has(%04b) = %t, want %t", tt.perms, tt.flag, got, tt.want)
			}
		})
	}
}
