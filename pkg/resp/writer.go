package resp

import (
	"bufio"
	"encoding"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"reflect"
	"strconv"
	"time"
)

type RESPVersion int

const (
	RESP2          RESPVersion = 2
	RESP3          RESPVersion = 3
	DefaultVersion             = RESP2
)

type writer interface {
	io.Writer
	io.ByteWriter

	WriteString(s string) (n int, err error)
}

type Writer struct {
	respVersion RESPVersion

	writer
	lenBuf []byte
	numBuf []byte
}

func NewWriter(w writer, respVersion RESPVersion) *Writer {
	return &Writer{
		respVersion: respVersion,
		writer:      w,
		lenBuf:      make([]byte, 0, 64),
		numBuf:      make([]byte, 0, 64),
	}
}

func (w *Writer) SetVersion(version RESPVersion) {
	w.respVersion = version
}

func (w *Writer) Reset() {
	if bw, ok := w.writer.(*bufio.Writer); ok {
		bw.Reset(w.writer)
	}
	w.lenBuf = w.lenBuf[:0]
	w.numBuf = w.numBuf[:0]
}

func (w *Writer) Flush() error {
	if bw, ok := w.writer.(*bufio.Writer); ok {
		n := bw.Buffered()
		if n > 0 {
			err := bw.Flush()
			if err != nil {
				return fmt.Errorf("failed to flush writer: %w", err)
			}
		}
	}
	return nil
}

func (w *Writer) WriteValue(value any) error {
	if w.respVersion == RESP2 {
		return w.writeResp2Value(value)
	}

	return w.writeResp3Value(value)
}

func (w *Writer) writeResp2Value(value any) error {
	switch v := value.(type) {
	case nil:
		return w.WriteString("")
	case string:
		return w.WriteString(v)
	case []byte:
		return w.writeBytesWithType(BulkString, v)
	case *string:
		return w.WriteString(*v)
	case int:
		return w.writeResp2Int(int64(v))
	case *int:
		return w.writeResp2Int(int64(*v))
	case int8:
		return w.writeResp2Int(int64(v))
	case *int8:
		return w.writeResp2Int(int64(*v))
	case int16:
		return w.writeResp2Int(int64(v))
	case *int16:
		return w.writeResp2Int(int64(*v))
	case int32:
		return w.writeResp2Int(int64(v))
	case *int32:
		return w.writeResp2Int(int64(*v))
	case int64:
		return w.writeResp2Int(v)
	case *int64:
		return w.writeResp2Int(*v)
	case uint:
		return w.writeResp2Uint(uint64(v))
	case *uint:
		return w.writeResp2Uint(uint64(*v))
	case uint8:
		return w.writeResp2Uint(uint64(v))
	case *uint8:
		return w.writeResp2Uint(uint64(*v))
	case uint16:
		return w.writeResp2Uint(uint64(v))
	case *uint16:
		return w.writeResp2Uint(uint64(*v))
	case uint32:
		return w.writeResp2Uint(uint64(v))
	case *uint32:
		return w.writeResp2Uint(uint64(*v))
	case uint64:
		return w.writeResp2Uint(uint64(v))
	case *uint64:
		return w.writeResp2Uint(uint64(*v))
	case float32:
		return w.writeResp2Float(float64(v))
	case *float32:
		return w.writeResp2Float(float64(*v))
	case float64:
		return w.writeResp2Float(v)
	case *float64:
		return w.writeResp2Float(*v)
	case bool:
		if v {
			return w.writeResp2Int(1)
		}
		return w.writeResp2Int(0)
	case *bool:
		if *v {
			return w.writeResp2Int(1)
		}
		return w.writeResp2Int(0)
	case time.Time:
		w.numBuf = v.AppendFormat(w.numBuf[:0], time.RFC3339Nano)
		return w.writeBytesWithType(BulkString, w.numBuf)
	case time.Duration:
		return w.writeResp2Int(v.Nanoseconds())
	case encoding.BinaryMarshaler:
		b, err := v.MarshalBinary()
		if err != nil {
			return err
		}
		return w.writeBytesWithType(BulkString, b)
	case net.IP:
		return w.WriteString(v.String())
	case error:
		return w.WriteSimpleValue(SimpleError, []byte(v.Error()))
	case RESPVersion:
		return w.writeInt(int64(v))
	default:
		if reflect.ValueOf(value).Kind() == reflect.Slice {
			return w.WriteSlice(value)
		}
		var rawMessage []byte
		err := json.NewEncoder(w).Encode(value)
		if err != nil {
			return fmt.Errorf("failed to encode value: %w", err)
		}
		return w.writeBytesWithType(BulkString, rawMessage)
	}
}

func (w *Writer) writeResp3Value(value any) error {
	switch v := value.(type) {
	case nil:
		return w.WriteSimpleValue(Null, []byte{})
	case []byte:
		return w.writeBytesWithType(BulkString, v)
	case string:
		return w.WriteString(v)
	case *string:
		return w.WriteString(*v)
	case int:
		return w.writeInt(int64(v))
	case *int:
		return w.writeInt(int64(*v))
	case int8:
		return w.writeInt(int64(v))
	case *int8:
		return w.writeInt(int64(*v))
	case int16:
		return w.writeInt(int64(v))
	case *int16:
		return w.writeInt(int64(*v))
	case int32:
		return w.writeInt(int64(v))
	case *int32:
		return w.writeInt(int64(*v))
	case int64:
		return w.writeInt(v)
	case *int64:
		return w.writeInt(*v)
	case uint:
		return w.writeUint(uint64(v))
	case *uint:
		return w.writeUint(uint64(*v))
	case uint8:
		return w.writeUint(uint64(v))
	case *uint8:
		return w.writeUint(uint64(*v))
	case uint16:
		return w.writeUint(uint64(v))
	case *uint16:
		return w.writeUint(uint64(*v))
	case uint32:
		return w.writeUint(uint64(v))
	case *uint32:
		return w.writeUint(uint64(*v))
	case uint64:
		return w.writeUint(uint64(v))
	case *uint64:
		return w.writeUint(uint64(*v))
	case float32:
		return w.writeFloat(float64(v))
	case *float32:
		return w.writeFloat(float64(*v))
	case float64:
		return w.writeFloat(v)
	case *float64:
		return w.writeFloat(*v)
	case bool:
		if v {
			return w.WriteSimpleValue(Boolean, []byte{'t'})
		}
		return w.WriteSimpleValue(Boolean, []byte{'f'})
	case *bool:
		if *v {
			return w.WriteSimpleValue(Boolean, []byte{'t'})
		}
		return w.WriteSimpleValue(Boolean, []byte{'f'})
	case time.Time:
		w.numBuf = v.AppendFormat(w.numBuf[:0], time.RFC3339Nano)
		return w.writeBytesWithType(BulkString, w.numBuf)
	case time.Duration:
		return w.writeInt(v.Nanoseconds())
	case encoding.BinaryMarshaler:
		b, err := v.MarshalBinary()
		if err != nil {
			return err
		}
		return w.writeBytesWithType(BulkString, b)
	case net.IP:
		return w.WriteString(v.String())
	case error:
		return w.writeBytesWithType(BulkError, []byte(v.Error()))
	case RESPVersion:
		return w.writeInt(int64(v))
	default:
		if reflect.ValueOf(value).Kind() == reflect.Slice {
			return w.WriteSlice(value)
		}
		var rawMessage []byte
		err := json.NewEncoder(w).Encode(value)
		if err != nil {
			return fmt.Errorf("failed to encode value: %w", err)
		}
		return w.writeBytesWithType(BulkString, rawMessage)
	}
}

func (w *Writer) writeResp2Int(n int64) error {
	w.numBuf = append(w.numBuf[:0], []byte(strconv.FormatInt(n, 10))...)
	return w.writeBytesWithType(BulkString, w.numBuf)
}

func (w *Writer) writeResp2Uint(n uint64) error {
	w.numBuf = append(w.numBuf[:0], []byte(strconv.FormatUint(n, 10))...)
	return w.writeBytesWithType(BulkString, w.numBuf)
}

func (w *Writer) writeResp2Float(f float64) error {
	w.numBuf = append(w.numBuf[:0], []byte(strconv.FormatFloat(f, 'f', -1, 64))...)
	return w.writeBytesWithType(BulkString, w.numBuf)
}

func (w *Writer) WriteStringSlice(v []string) error {
	if err := w.WriteByte(Array); err != nil {
		return err
	}
	if err := w.writeLen(len(v)); err != nil {
		return err
	}
	for i := range v {
		if err := w.WriteString(v[i]); err != nil {
			return err
		}
	}
	return nil
}

func (w *Writer) WriteSlice(v any) error {
	reflectType := reflect.TypeOf(v)
	if reflectType.Kind() == reflect.Ptr {
		reflectType = reflectType.Elem()
	}

	if reflectType.Kind() != reflect.Slice {
		return fmt.Errorf("expected slice, got %s", reflectType.Kind())
	}

	if err := w.WriteByte(Array); err != nil {
		return err
	}
	values := reflect.ValueOf(v)
	sLen := values.Len()
	if err := w.writeLen(sLen); err != nil {
		return err
	}
	for i := 0; i < sLen; i++ {
		if err := w.WriteValue(values.Index(i).Interface()); err != nil {
			return err
		}
	}

	return nil
}

func (w *Writer) WriteMapOrdered(m map[string]any, orderedKeys []string) error {
	if err := w.WriteByte(Map); err != nil {
		return err
	}

	if err := w.writeLen(len(m)); err != nil {
		return err
	}

	for _, k := range orderedKeys {
		v, ok := m[k]
		if !ok {
			continue // Skip keys that are not in the map
		}
		if err := w.writeBytesWithType(BulkString, []byte(k)); err != nil {
			return err
		}
		if err := w.WriteValue(v); err != nil {
			return err
		}
	}

	return nil
}

func (w *Writer) WriteMap(m map[string]any) error {
	if err := w.WriteByte(Map); err != nil {
		return err
	}

	if err := w.writeLen(len(m)); err != nil {
		return err
	}

	for k, v := range m {
		if err := w.writeBytesWithType(BulkString, []byte(k)); err != nil {
			return err
		}
		if err := w.WriteValue(v); err != nil {
			return err
		}
	}

	return nil
}

func (w *Writer) WriteNull(valType byte) error {
	if w.respVersion == RESP2 {
		if err := w.WriteByte(valType); err != nil {
			return err
		}

		if err := w.writeLen(-1); err != nil {
			return err
		}

		return nil
	}
	return w.WriteSimpleValue(Null, []byte{})
}

func (w *Writer) WriteError(err error) error {
	return w.WriteValue(err)
}

func (w *Writer) WriteSimpleValue(valType byte, value []byte) error {
	if err := w.WriteByte(valType); err != nil {
		return err
	}
	if _, err := w.Write(value); err != nil {
		return err
	}
	return w.writeCLRF()
}

func (w *Writer) writeLen(n int) error {
	w.lenBuf = append(w.lenBuf[:0], []byte(strconv.Itoa(n))...)
	w.lenBuf = append(w.lenBuf, '\r', '\n')
	_, err := w.Write(w.lenBuf)
	return err
}

func (w *Writer) writeInt(n int64) error {
	w.numBuf = append(w.numBuf[:0], []byte(strconv.FormatInt(n, 10))...)
	return w.WriteSimpleValue(Integer, w.numBuf)
}

func (w *Writer) writeUint(n uint64) error {
	w.numBuf = append(w.numBuf[:0], []byte(strconv.FormatUint(n, 10))...)
	return w.WriteSimpleValue(Integer, w.numBuf)
}

func (w *Writer) writeFloat(f float64) error {
	w.numBuf = append(w.numBuf[:0], []byte(strconv.FormatFloat(f, 'f', -1, 64))...)
	return w.WriteSimpleValue(Double, w.numBuf)
}

func (w *Writer) WriteString(s string) error {
	return w.writeBytesWithType(BulkString, []byte(s))
}

func (w *Writer) WriteBytes(b []byte) error {
	return w.writeBytesWithType(BulkString, b)
}

func (w *Writer) writeBytesWithType(valType byte, b []byte) error {
	if err := w.WriteByte(valType); err != nil {
		return err
	}

	bLen := len(b)
	if err := w.writeLen(bLen); err != nil {
		return err
	}

	if _, err := w.Write(b); err != nil {
		return err
	}
	return w.writeCLRF()
}

func (w *Writer) writeCLRF() error {
	if err := w.WriteByte('\r'); err != nil {
		return err
	}
	return w.WriteByte('\n')
}
