package service

import (
	"context"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/go-redis/redis/v8"
	"github.com/samber/hot"
)

const adaptiveDecisionTTL = 15 * time.Minute

type AdaptiveReasoningDecision struct {
	Effort      string         `json:"effort"`
	Generations int            `json:"generations"`
	Remaining   int            `json:"remaining"`
	UserHash    string         `json:"user_hash"`
	FailureHash string         `json:"failure_hash"`
	RequestID   string         `json:"request_id"`
	Answers     map[string]any `json:"answers,omitempty"`
}

var adaptiveDecisions = hot.NewHotCache[string, AdaptiveReasoningDecision](hot.LRU, 20000).Build()
var adaptiveDecisionsMu sync.Mutex

// 消耗计数在 Redis 内原子执行，多个网关实例不会重复使用同一次剩余额度。
var takeAdaptiveDecision = redis.NewScript(`
local raw = redis.call('GET', KEYS[1])
if not raw then return nil end
local ok, entry = pcall(cjson.decode, raw)
if not ok then return nil end
if entry.remaining <= 0 or (ARGV[1] ~= '' and entry.user_hash ~= ARGV[1]) or (ARGV[2] ~= '' and entry.failure_hash ~= ARGV[2]) then return nil end
entry.remaining = entry.remaining - 1
local encoded = cjson.encode(entry)
local ttl = redis.call('PTTL', KEYS[1])
if ttl <= 0 then return nil end
redis.call('SET', KEYS[1], encoded, 'PX', ttl)
return encoded
`)

func TakeAdaptiveReasoningDecision(ctx context.Context, key, userHash, failureHash string) (AdaptiveReasoningDecision, bool) {
	key = "new-api:adaptive-reasoning:v1:" + key
	if common.RedisEnabled && common.RDB != nil {
		ctx, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
		defer cancel()
		encoded, err := takeAdaptiveDecision.Run(ctx, common.RDB, []string{key}, userHash, failureHash).Text()
		var value AdaptiveReasoningDecision
		if err != nil || common.UnmarshalJsonStr(encoded, &value) != nil {
			return value, false
		}
		return value, true
	}
	adaptiveDecisionsMu.Lock()
	defer adaptiveDecisionsMu.Unlock()
	value, found, err := adaptiveDecisions.Get(key)
	if err != nil || !found || value.Remaining <= 0 || userHash != "" && value.UserHash != userHash || failureHash != "" && value.FailureHash != failureHash {
		return AdaptiveReasoningDecision{}, false
	}
	value.Remaining--
	adaptiveDecisions.SetWithTTL(key, value, adaptiveDecisionTTL)
	return value, true
}

func StoreAdaptiveReasoningDecision(ctx context.Context, key string, value AdaptiveReasoningDecision) {
	key = "new-api:adaptive-reasoning:v1:" + key
	if common.RedisEnabled && common.RDB != nil {
		encoded, err := common.Marshal(value)
		if err != nil {
			return
		}
		ctx, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
		defer cancel()
		_ = common.RDB.Set(ctx, key, string(encoded), adaptiveDecisionTTL).Err()
		return
	}
	adaptiveDecisionsMu.Lock()
	defer adaptiveDecisionsMu.Unlock()
	adaptiveDecisions.SetWithTTL(key, value, adaptiveDecisionTTL)
}

func ClearAdaptiveReasoningDecision(ctx context.Context, key string) {
	key = "new-api:adaptive-reasoning:v1:" + key
	if common.RedisEnabled && common.RDB != nil {
		ctx, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
		defer cancel()
		_ = common.RDB.Del(ctx, key).Err()
		return
	}
	adaptiveDecisionsMu.Lock()
	defer adaptiveDecisionsMu.Unlock()
	adaptiveDecisions.Delete(key)
}
