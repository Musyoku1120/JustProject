package frame

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"strings"
)

type RedisManager struct {
	*redis.Client
	ctx context.Context
}

type RedisConfig struct {
	Addr     string
	Password string
}

type PubSubHandler func(channel string, message string)

func NewRedisManager(configStr string) (*RedisManager, error) {
	ctx := context.Background()

	config := strings.Split(configStr, "@")
	client := redis.NewClient(&redis.Options{
		Addr:     config[0],
		Password: config[1],
		PoolSize: 20, // 连接池大小
	})

	// 测试连接
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisManager{Client: client, ctx: ctx}, nil
}

// Subscribe 订阅单个频道
func (m *RedisManager) Subscribe(channel string, handler PubSubHandler) (func() error, error) {
	pubSub := m.Client.Subscribe(m.ctx, channel)

	// 确认订阅
	if _, err := pubSub.Receive(m.ctx); err != nil {
		return nil, err
	}

	// 启动goroutine处理消息
	stopChan := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopChan:
				return
			default:
				msg, err := pubSub.ReceiveMessage(m.ctx)
				if err != nil {
					// 处理错误（如连接断开）
					return
				}
				handler(msg.Channel, msg.Payload)
			}
		}
	}()

	// 返回取消订阅函数
	return func() error {
		close(stopChan)
		return pubSub.Close()
	}, nil
}

// PSubscribe 模式订阅
func (m *RedisManager) PSubscribe(pattern string, handler PubSubHandler) (func() error, error) {
	pubSub := m.Client.PSubscribe(m.ctx, pattern)

	// 确认订阅
	if _, err := pubSub.Receive(m.ctx); err != nil {
		return nil, err
	}

	// 启动goroutine处理消息
	stopChan := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopChan:
				return
			default:
				msg, err := pubSub.ReceiveMessage(m.ctx)
				if err != nil {
					// 处理错误
					return
				}
				handler(msg.Channel, msg.Payload)
			}
		}
	}()

	// 返回取消订阅函数
	return func() error {
		close(stopChan)
		return pubSub.Close()
	}, nil
}

// Publish 发布消息
func (m *RedisManager) Publish(channel string, message interface{}) (int64, error) {
	return m.Client.Publish(m.ctx, channel, message).Result()
}

// Transaction 执行Redis事务
func (m *RedisManager) Transaction(fn func(tx *redis.Tx) error, keys ...string) error {
	return m.Client.Watch(m.ctx, fn, keys...)
}

// WithContext 使用自定义上下文
func (m *RedisManager) WithContext(ctx context.Context) *RedisManager {
	return &RedisManager{
		Client: m.Client,
		ctx:    ctx,
	}
}

// HealthCheck 健康检查
func (m *RedisManager) HealthCheck() error {
	if err := m.Client.Ping(m.ctx).Err(); err != nil {
		return fmt.Errorf("redis health check failed: %w", err)
	}
	return nil
}

// Info 获取Redis服务器信息
func (m *RedisManager) Info(section ...string) (string, error) {
	return m.Client.Info(m.ctx, section...).Result()
}
