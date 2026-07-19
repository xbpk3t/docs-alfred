package mail

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultFrom(t *testing.T) {
	require.Equal(t, "noreply <onboarding@resend.dev>", DefaultFrom(""))
	require.Equal(t, "noreply <onboarding@resend.dev>", DefaultFrom("  "))
	require.Equal(t, "wiki compact <onboarding@resend.dev>", DefaultFrom("wiki compact"))
	require.Equal(t, "Linear Bot <onboarding@resend.dev>", DefaultFrom("Linear Bot"))
}

func TestParseAddresses(t *testing.T) {
	require.Nil(t, ParseAddresses(""))
	require.Nil(t, ParseAddresses("  "))
	require.Equal(t, []string{"a@b.c"}, ParseAddresses("a@b.c"))
	require.Equal(t, []string{"a@b.c", "d@e.f"}, ParseAddresses(" a@b.c , d@e.f , "))
}

func TestSendHTML_Validation(t *testing.T) {
	ctx := context.Background()
	base := SendOptions{
		Token:   "tok",
		To:      []string{"a@b.c"},
		From:    DefaultFrom("x"),
		Subject: "s",
		HTML:    "<p>hi</p>",
	}

	t.Run("missing token", func(t *testing.T) {
		o := base
		o.Token = ""
		require.Error(t, SendHTML(ctx, &o))
	})
	t.Run("missing to", func(t *testing.T) {
		o := base
		o.To = nil
		require.Error(t, SendHTML(ctx, &o))
	})
	t.Run("missing from", func(t *testing.T) {
		o := base
		o.From = ""
		require.Error(t, SendHTML(ctx, &o))
	})
	t.Run("missing subject", func(t *testing.T) {
		o := base
		o.Subject = ""
		require.Error(t, SendHTML(ctx, &o))
	})
	t.Run("nil opts", func(t *testing.T) {
		require.Error(t, SendHTML(ctx, nil))
	})
}
