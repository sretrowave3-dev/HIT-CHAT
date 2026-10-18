package ephemeral

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hit-chat/campusnet/pkg/protocol"
	"github.com/redis/go-redis/v9"
)

// BurnLuaScript is an atomic Redis script that overwrites the key and unlinks it.
const BurnLuaScript = `
local key = KEYS[1]
local exists = redis.call('EXISTS', key)
if exists == 1 then
    redis.call('SET', key, '')
    redis.call('UNLINK', key)
    return 1
else
    return 0
end
`

// RedisStore provides Redis-backed ephemeral envelope storage with millisecond TTL.
type RedisStore struct {
	client     *redis.Client
	burnScript *redis.Script
	keyPrefix  string
}

// NewRedisStore initializes a Redis ephemeral store.
func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{
		client:     client,
		burnScript: redis.NewScript(BurnLuaScript),
		keyPrefix:  "campusnet:env:",
	}
}

func (r *RedisStore) formatKey(id string) string {
	return r.keyPrefix + id
}

func (r *RedisStore) StoreEnvelope(ctx context.Context, env *protocol.Envelope) error {
	data, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("serializing envelope: %w", err)
	}

	key := r.formatKey(env.ID)
	ttl := time.Duration(env.TTLMilliseconds) * time.Millisecond
	if ttl <= 0 {
		ttl = 10 * time.Second
	}

	// PSETEX stores with millisecond precision
	if err := r.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("redis store error: %w", err)
	}
	return nil
}

func (r *RedisStore) GetEnvelope(ctx context.Context, id string) (*protocol.Envelope, error) {
	key := r.formatKey(id)
	val, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrEnvelopeNotFound
		}
		return nil, err
	}

	var env protocol.Envelope
	if err := json.Unmarshal(val, &env); err != nil {
		return nil, fmt.Errorf("deserializing envelope: %w", err)
	}

	return &env, nil
}

func (r *RedisStore) BurnEnvelope(ctx context.Context, id string) error {
	key := r.formatKey(id)
	// Execute atomic unlink & overwrite script
	err := r.burnScript.Run(ctx, r.client, []string{key}).Err()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("burn script failed: %w", err)
	}
	return nil
}

func (r *RedisStore) Close() error {
	return r.client.Close()
}
