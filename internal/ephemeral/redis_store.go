package ephemeral

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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

	// Index message under recipient's inbox set for non-blocking O(1) retrieval
	cleanRecipient := strings.TrimPrefix(strings.ToLower(env.RecipientID), "@")
	inboxKey := fmt.Sprintf("%sinbox:%s", r.keyPrefix, cleanRecipient)
	_ = r.client.SAdd(ctx, inboxKey, env.ID).Err()
	_ = r.client.Expire(ctx, inboxKey, 24*time.Hour).Err()

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

func (r *RedisStore) GetPendingEnvelopesForRecipient(ctx context.Context, recipient string) ([]*protocol.Envelope, error) {
	cleanRecipient := strings.TrimPrefix(strings.ToLower(recipient), "@")
	aliases := []string{cleanRecipient}
	if strings.HasPrefix(cleanRecipient, "student_") {
		aliases = append(aliases, strings.TrimPrefix(cleanRecipient, "student_"))
	} else {
		aliases = append(aliases, "student_"+cleanRecipient)
	}

	var result []*protocol.Envelope
	seenIDs := make(map[string]bool)

	// 1. Efficient non-blocking index lookup via Redis Set
	for _, alias := range aliases {
		inboxKey := fmt.Sprintf("%sinbox:%s", r.keyPrefix, alias)
		msgIDs, err := r.client.SMembers(ctx, inboxKey).Result()
		if err == nil {
			for _, mid := range msgIDs {
				if seenIDs[mid] {
					continue
				}
				seenIDs[mid] = true
				env, err := r.GetEnvelope(ctx, mid)
				if err == nil && !env.IsExpired() {
					result = append(result, env)
				} else {
					// Stale or burned message, evict from set
					_ = r.client.SRem(ctx, inboxKey, mid).Err()
				}
			}
		}
	}

	// 2. If no messages found in index, fallback to non-blocking SCAN instead of blocking KEYS *
	if len(result) == 0 {
		var cursor uint64
		maxScan := 100
		scanned := 0
		for {
			var keys []string
			var err error
			keys, cursor, err = r.client.Scan(ctx, cursor, r.keyPrefix+"*", 25).Result()
			if err != nil {
				break
			}
			for _, k := range keys {
				if strings.Contains(k, "inbox:") {
					continue
				}
				val, gErr := r.client.Get(ctx, k).Bytes()
				if gErr == nil {
					var env protocol.Envelope
					if err := json.Unmarshal(val, &env); err == nil && !env.IsExpired() {
						target := strings.TrimPrefix(strings.ToLower(env.RecipientID), "@")
						target = strings.TrimPrefix(target, "student_")
						cleaned := strings.TrimPrefix(cleanRecipient, "student_")
						if target == cleaned && !seenIDs[env.ID] {
							seenIDs[env.ID] = true
							result = append(result, &env)
						}
					}
				}
			}
			scanned += len(keys)
			if cursor == 0 || scanned >= maxScan {
				break
			}
		}
	}

	return result, nil
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
