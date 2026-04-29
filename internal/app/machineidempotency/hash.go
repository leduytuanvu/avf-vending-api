package machineidempotency

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"

	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// StableProtoHash returns a SHA-256 over deterministic protobuf serialization (stable field order).
func StableProtoHash(msg proto.Message) ([]byte, error) {
	if msg == nil {
		return nil, errors.New("machineidempotency: nil message")
	}
	b, err := proto.MarshalOptions{Deterministic: true}.Marshal(msg)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(b)
	return sum[:], nil
}

// HashMutationRequest fingerprints a unary machine mutation for the idempotency ledger.
// Unstable identifiers (currently MachineRequestMeta.request_id anywhere in the tree) are cleared
// so retries may vary observability/trace fields without splitting the ledger entry.
func HashMutationRequest(msg proto.Message) ([]byte, error) {
	if msg == nil {
		return nil, errors.New("machineidempotency: nil message")
	}
	cl := proto.Clone(msg)
	scrubRequestIDsForLedger(cl.ProtoReflect())
	return StableProtoHash(cl)
}

func scrubRequestIDsForLedger(msg protoreflect.Message) {
	if !msg.IsValid() {
		return
	}
	fields := msg.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		if fd.Cardinality() == protoreflect.Repeated {
			continue
		}
		if !msg.Has(fd) {
			continue
		}
		switch fd.Kind() {
		case protoreflect.MessageKind:
			scrubRequestIDsForLedger(msg.Get(fd).Message())
		case protoreflect.StringKind:
			if string(fd.Name()) == "request_id" {
				msg.Clear(fd)
			}
		}
	}
}

// MachineRequestIdempotencyKey searches nested messages for the first non-empty idempotency_key string field.
func MachineRequestIdempotencyKey(msg proto.Message) string {
	if msg == nil {
		return ""
	}
	var out string
	inspectIdempotencyKey(msg.ProtoReflect(), &out)
	return out
}

func inspectIdempotencyKey(msg protoreflect.Message, out *string) {
	if !msg.IsValid() || out == nil || *out != "" {
		return
	}
	fields := msg.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		if fd.Cardinality() == protoreflect.Repeated {
			continue
		}
		if fd.Kind() == protoreflect.StringKind && string(fd.Name()) == "idempotency_key" && msg.Has(fd) {
			*out = strings.TrimSpace(msg.Get(fd).String())
			return
		}
		if fd.Kind() == protoreflect.MessageKind && msg.Has(fd) {
			inspectIdempotencyKey(msg.Get(fd).Message(), out)
		}
	}
}

// MutationIdempotencyKey returns the ledger idempotency string for unary machine mutations.
//
// Normal requests carry idempotency_key on IdempotencyContext / nested enums; reconcile batches derive
// a stable key from a sorted fingerprint of reconcile idempotency keys.
func MutationIdempotencyKey(msg proto.Message) (string, error) {
	if msg == nil {
		return "", errors.New("machineidempotency: nil message")
	}
	if r, ok := msg.(*machinev1.ReconcileEventsRequest); ok {
		keys := r.GetIdempotencyKeys()
		if len(keys) < 1 || len(keys) > 500 {
			return "", fmt.Errorf("machineidempotency: reconcile idempotency_keys must contain 1 to 500 entries")
		}
		for _, k := range keys {
			if strings.TrimSpace(k) == "" {
				return "", fmt.Errorf("machineidempotency: reconcile idempotency_keys must not include empty strings")
			}
		}
		return reconcileEventsSyntheticKey(keys), nil
	}
	key := MachineRequestIdempotencyKey(msg)
	if strings.TrimSpace(key) == "" {
		return "", errors.New("machineidempotency: idempotency_key required")
	}
	return key, nil
}

func reconcileEventsSyntheticKey(keys []string) string {
	cp := append([]string(nil), keys...)
	for i := range cp {
		cp[i] = strings.TrimSpace(cp[i])
	}
	sort.Strings(cp)
	h := sha256.New()
	for _, k := range cp {
		h.Write([]byte(k))
		h.Write([]byte{0})
	}
	return "telemetry.reconcile.v1.sha256-" + hex.EncodeToString(h.Sum(nil))
}
