package errors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedfishError(t *testing.T) {
	t.Run("Error message format", func(t *testing.T) {
		err := NewRedfishError("192.168.1.10", "/redfish/v1/Systems", 401, "Unauthorized", "Invalid credentials")

		assert.Contains(t, err.Error(), "192.168.1.10")
		assert.Contains(t, err.Error(), "/redfish/v1/Systems")
		assert.Contains(t, err.Error(), "401")
	})

	t.Run("IsAuthError", func(t *testing.T) {
		err401 := NewRedfishError("host", "/path", 401, "Unauthorized", "")
		err403 := NewRedfishError("host", "/path", 403, "Forbidden", "")
		err500 := NewRedfishError("host", "/path", 500, "Internal Server Error", "")

		assert.True(t, err401.IsAuthError())
		assert.True(t, err403.IsAuthError())
		assert.False(t, err500.IsAuthError())
	})

	t.Run("IsNotFound", func(t *testing.T) {
		err404 := NewRedfishError("host", "/path", 404, "Not Found", "")
		err500 := NewRedfishError("host", "/path", 500, "Internal Server Error", "")

		assert.True(t, err404.IsNotFound())
		assert.False(t, err500.IsNotFound())
	})
}

func TestCollectionError(t *testing.T) {
	t.Run("Error message format", func(t *testing.T) {
		innerErr := errors.New("timeout")
		err := NewCollectionError("192.168.1.10", "processors", innerErr)

		assert.Contains(t, err.Error(), "192.168.1.10")
		assert.Contains(t, err.Error(), "processors")
		assert.Contains(t, err.Error(), "timeout")
	})

	t.Run("Unwrap", func(t *testing.T) {
		innerErr := ErrTimeout
		err := NewCollectionError("host", "memory", innerErr)

		assert.True(t, errors.Is(err, ErrTimeout))
	})
}

func TestConfigError(t *testing.T) {
	err := NewConfigError("servers[0].host", "host is required")

	assert.Contains(t, err.Error(), "servers[0].host")
	assert.Contains(t, err.Error(), "host is required")
}

func TestMultiError(t *testing.T) {
	t.Run("Empty MultiError", func(t *testing.T) {
		me := &MultiError{}

		assert.False(t, me.HasErrors())
		assert.Nil(t, me.ErrorOrNil())
		assert.Equal(t, "no errors", me.Error())
	})

	t.Run("Single error", func(t *testing.T) {
		me := &MultiError{}
		me.Add(errors.New("first error"))

		assert.True(t, me.HasErrors())
		assert.NotNil(t, me.ErrorOrNil())
		assert.Equal(t, "first error", me.Error())
	})

	t.Run("Multiple errors", func(t *testing.T) {
		me := &MultiError{}
		me.Add(errors.New("first error"))
		me.Add(errors.New("second error"))
		me.Add(errors.New("third error"))

		assert.True(t, me.HasErrors())
		assert.Contains(t, me.Error(), "3 errors occurred")
		assert.Contains(t, me.Error(), "first error")
	})

	t.Run("Add nil error", func(t *testing.T) {
		me := &MultiError{}
		me.Add(nil)

		assert.False(t, me.HasErrors())
	})

	t.Run("Is checks all errors", func(t *testing.T) {
		me := &MultiError{}
		me.Add(errors.New("unrelated"))
		me.Add(ErrTimeout)
		me.Add(errors.New("another"))

		assert.True(t, errors.Is(me, ErrTimeout))
		assert.False(t, errors.Is(me, ErrNotFound))
	})
}

func TestSentinelErrors(t *testing.T) {
	// Ensure sentinel errors are distinct
	errs := []error{
		ErrConnectionFailed,
		ErrAuthenticationFailed,
		ErrTimeout,
		ErrNotFound,
		ErrInvalidResponse,
		ErrConfigInvalid,
		ErrNoServers,
	}

	for i, err1 := range errs {
		for j, err2 := range errs {
			if i != j {
				assert.False(t, errors.Is(err1, err2), "errors %v and %v should not match", err1, err2)
			}
		}
	}
}
