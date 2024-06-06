package resp

import (
	"bufio"
	"encoding"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"
)

type writer interface {
	io.Writer
	io.ByteWriter

	WriteString(s string) (n int, err error)
}

type Writer struct {
	writer

	lenBuf []byte
	numBuf []byte
}

func NewWriter(w writer) *Writer {
	return &Writer{
		writer: w,
		lenBuf: make([]byte, 0, 64),
		numBuf: make([]byte, 0, 64),
	}
}

func (w *Writer) Flush() error {
	if bw, ok := w.writer.(*bufio.Writer); ok {
		return bw.Flush()
	}
	return nil
}

func (w *Writer) WriteArray(values []any) error {
	if err := w.WriteByte(Array); err != nil {
		return err
	}

	if err := w.writeLen(len(values)); err != nil {
		return err
	}

	for _, value := range values {
		if err := w.WriteValue(value); err != nil {
			return err
		}
	}

	return nil
}

func (w *Writer) WriteValue(value any) error {
	switch v := value.(type) {
	case nil:
		return w.WriteSimpleValue(Null, []byte{})
	case string:
		return w.writeString(v)
	case []byte:
		return w.writeBytes(BulkString, v)
	case *string:
		return w.writeString(*v)
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
		return w.writeBytes(BulkString, w.numBuf)
	case time.Duration:
		return w.writeInt(v.Nanoseconds())
	case encoding.BinaryMarshaler:
		b, err := v.MarshalBinary()
		if err != nil {
			return err
		}
		return w.writeBytes(BulkString, b)
	case net.IP:
		return w.writeString(v.String())
	case error:
		return w.writeBytes(BulkError, []byte(v.Error()))
	default:
		return fmt.Errorf("cannot marshal %T", value)
	}
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

func (w *Writer) writeString(s string) error {
	return w.writeBytes(BulkString, []byte(s))
}

func (w *Writer) writeBytes(valType byte, b []byte) error {
	if err := w.WriteByte(valType); err != nil {
		return err
	}

	if err := w.writeLen(len(b)); err != nil {
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
