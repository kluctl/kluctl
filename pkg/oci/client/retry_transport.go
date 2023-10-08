/*
Copyright 2023 The Flux authors
Copyright 2018 Google LLC All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"syscall"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"

	"github.com/fluxcd/pkg/oci"
)

// WithRetryTransport returns a crane.Option for setting transport that uses the  backoff for retries
//
// Most parts(including the functions below) are copied from https://github.com/google/go-containerregistry/blob/v0.14.0/pkg/v1/remote/options.go#L152
// so we have the same transport used in the library but with a different retry backoff.
func WithRetryTransport(ctx context.Context, ref name.Reference, auth authn.Authenticator, backoff remote.Backoff, scopes []string) (crane.Option, error) {
	var retryTransport http.RoundTripper
	retryTransport = remote.DefaultTransport.(*http.Transport).Clone()
	if logs.Enabled(logs.Debug) {
		retryTransport = transport.NewLogger(retryTransport)
	}
	retryTransport = transport.NewRetry(retryTransport,
		transport.WithRetryPredicate(defaultRetryPredicate),
		transport.WithRetryStatusCodes(retryableStatusCodes...),
		transport.WithRetryBackoff(backoff))
	retryTransport = transport.NewUserAgent(retryTransport, oci.UserAgent)

	t, err := transport.NewWithContext(ctx, ref.Context().Registry, auth, retryTransport, scopes)
	if err != nil {
		return nil, err
	}
	return crane.WithTransport(t), nil
}

var defaultRetryPredicate = func(err error) bool {
	// Various failure modes here, as we're often reading from and writing to
	// the network.
	if isTemporary(err) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) || errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET) {
		logs.Warn.Printf("retrying %v", err)
		return true
	}
	return false
}

type temporary interface {
	Temporary() bool
}

// isTemporary returns true if err implements Temporary() and it returns true.
func isTemporary(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if te, ok := err.(temporary); ok && te.Temporary() {
		return true
	}
	return false
}

var retryableStatusCodes = []int{
	http.StatusRequestTimeout,
	http.StatusInternalServerError,
	http.StatusBadGateway,
	http.StatusServiceUnavailable,
	http.StatusGatewayTimeout,
}
