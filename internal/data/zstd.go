package data

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"

	"github.com/klauspost/compress/zstd"
	"gorm.io/gorm/schema"
)

type zstdSerializer struct {
	encoder *zstd.Encoder
	decoder *zstd.Decoder
}

var registerZstdSerializer = sync.OnceValue(func() error {
	encoder, err := zstd.NewWriter(nil,
		zstd.WithEncoderLevel(zstd.SpeedBetterCompression),
		zstd.WithEncoderConcurrency(1),
	)
	if err != nil {
		return fmt.Errorf("create zstd encoder: %w", err)
	}
	decoder, err := zstd.NewReader(nil, zstd.WithDecoderConcurrency(1))
	if err != nil {
		return fmt.Errorf("create zstd decoder: %w", err)
	}
	schema.RegisterSerializer("zstd", &zstdSerializer{encoder: encoder, decoder: decoder})
	return nil
})

func (s *zstdSerializer) Scan(ctx context.Context, field *schema.Field, dst reflect.Value, dbValue any) error {
	zstdFrameMagic := []byte{0x28, 0xb5, 0x2f, 0xfd}

	fieldValue := reflect.New(field.FieldType)
	if dbValue == nil {
		field.ReflectValueOf(ctx, dst).Set(fieldValue.Elem())
		return nil
	}

	var raw []byte
	switch value := dbValue.(type) {
	case []byte:
		raw = value
	case string:
		raw = []byte(value)
	default:
		return fmt.Errorf("unsupported zstd database value %T", dbValue)
	}

	if bytes.HasPrefix(raw, zstdFrameMagic) {
		decoded, err := s.decoder.DecodeAll(raw, nil)
		if err != nil {
			return fmt.Errorf("decode zstd value: %w", err)
		}
		raw = decoded
	}

	if field.FieldType.Kind() == reflect.String {
		fieldValue.Elem().SetString(string(raw))
	} else if len(raw) > 0 {
		if err := json.Unmarshal(raw, fieldValue.Interface()); err != nil {
			return fmt.Errorf("decode zstd json value: %w", err)
		}
	}

	field.ReflectValueOf(ctx, dst).Set(fieldValue.Elem())
	return nil
}

func (s *zstdSerializer) Value(_ context.Context, field *schema.Field, _ reflect.Value, fieldValue any) (any, error) {
	var raw []byte
	if field.FieldType.Kind() == reflect.String {
		raw = []byte(reflect.ValueOf(fieldValue).String())
	} else {
		encoded, err := json.Marshal(fieldValue)
		if err != nil {
			return nil, fmt.Errorf("encode zstd json value: %w", err)
		}
		raw = encoded
	}

	compressed := s.encoder.EncodeAll(raw, nil)
	if len(compressed) < len(raw) {
		return compressed, nil
	}
	return raw, nil
}
