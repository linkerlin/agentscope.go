package retry

import (
	"context"
	"errors"
	"time"
)

// Options 控制重试行为
type Options struct {
	MaxAttempts int           // 至少为 1
	Backoff     time.Duration // 第 i 次重试前等待（线性：i * Backoff）；为 0 则不等待
}

// Do 在 fn 返回错误时按 Options 重试，ctx 取消时立即返回
func Do(ctx context.Context, opts Options, fn func() error) error {
	if opts.MaxAttempts < 1 {
		opts.MaxAttempts = 1
	}
	var err error
	for attempt := 1; attempt <= opts.MaxAttempts; attempt++ {
		if err = ctx.Err(); err != nil {
			return err
		}
		err = fn()
		if err == nil {
			return nil
		}
		if IsPermanent(err) {
			return err
		}
		if attempt == opts.MaxAttempts {
			break
		}
		if opts.Backoff > 0 {
			d := time.Duration(attempt) * opts.Backoff
			t := time.NewTimer(d)
			select {
			case <-ctx.Done():
				t.Stop()
				return ctx.Err()
			case <-t.C:
			}
		}
	}
	return err
}

// Permanent 将错误包装为不可重试错误
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return &permanentError{err: err}
}

// IsPermanent 判断是否为不可重试错误
func IsPermanent(err error) bool {
	var pe *permanentError
	return errors.As(err, &pe)
}

type permanentError struct {
	err error
}

func (e *permanentError) Error() string { return e.err.Error() }
func (e *permanentError) Unwrap() error { return e.err }
