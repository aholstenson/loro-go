package loro

// #include <loro.h>
import "C"

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"
)

// This is needed, because as of go 1.24
// type RustBuffer C.RustBuffer cannot have methods,
// RustBuffer is treated as non-local type
type GoRustBuffer struct {
	inner C.RustBuffer
}

type RustBufferI interface {
	AsReader() *bytes.Reader
	Free()
	ToGoBytes() []byte
	Data() unsafe.Pointer
	Len() uint64
	Capacity() uint64
}

func RustBufferFromExternal(b RustBufferI) GoRustBuffer {
	return GoRustBuffer{
		inner: C.RustBuffer{
			capacity: C.uint64_t(b.Capacity()),
			len:      C.uint64_t(b.Len()),
			data:     (*C.uchar)(b.Data()),
		},
	}
}

func (cb GoRustBuffer) Capacity() uint64 {
	return uint64(cb.inner.capacity)
}

func (cb GoRustBuffer) Len() uint64 {
	return uint64(cb.inner.len)
}

func (cb GoRustBuffer) Data() unsafe.Pointer {
	return unsafe.Pointer(cb.inner.data)
}

func (cb GoRustBuffer) AsReader() *bytes.Reader {
	b := unsafe.Slice((*byte)(cb.inner.data), C.uint64_t(cb.inner.len))
	return bytes.NewReader(b)
}

func (cb GoRustBuffer) Free() {
	rustCall(func(status *C.RustCallStatus) bool {
		C.ffi_loro_rustbuffer_free(cb.inner, status)
		return false
	})
}

func (cb GoRustBuffer) ToGoBytes() []byte {
	return C.GoBytes(unsafe.Pointer(cb.inner.data), C.int(cb.inner.len))
}

func stringToRustBuffer(str string) C.RustBuffer {
	return bytesToRustBuffer([]byte(str))
}

func bytesToRustBuffer(b []byte) C.RustBuffer {
	if len(b) == 0 {
		return C.RustBuffer{}
	}
	// We can pass the pointer along here, as it is pinned
	// for the duration of this call
	foreign := C.ForeignBytes{
		len:  C.int(len(b)),
		data: (*C.uchar)(unsafe.Pointer(&b[0])),
	}

	return rustCall(func(status *C.RustCallStatus) C.RustBuffer {
		return C.ffi_loro_rustbuffer_from_bytes(foreign, status)
	})
}

type BufLifter[GoType any] interface {
	Lift(value RustBufferI) GoType
}

type BufLowerer[GoType any] interface {
	Lower(value GoType) C.RustBuffer
}

type BufReader[GoType any] interface {
	Read(reader io.Reader) GoType
}

type BufWriter[GoType any] interface {
	Write(writer io.Writer, value GoType)
}

func LowerIntoRustBuffer[GoType any](bufWriter BufWriter[GoType], value GoType) C.RustBuffer {
	// This might be not the most efficient way but it does not require knowing allocation size
	// beforehand
	var buffer bytes.Buffer
	bufWriter.Write(&buffer, value)

	bytes, err := io.ReadAll(&buffer)
	if err != nil {
		panic(fmt.Errorf("reading written data: %w", err))
	}
	return bytesToRustBuffer(bytes)
}

func LiftFromRustBuffer[GoType any](bufReader BufReader[GoType], rbuf RustBufferI) GoType {
	defer rbuf.Free()
	reader := rbuf.AsReader()
	item := bufReader.Read(reader)
	if reader.Len() > 0 {
		// TODO: Remove this
		leftover, _ := io.ReadAll(reader)
		panic(fmt.Errorf("Junk remaining in buffer after lifting: %s", string(leftover)))
	}
	return item
}

func rustCallWithError[E any, U any](converter BufReader[*E], callback func(*C.RustCallStatus) U) (U, *E) {
	var status C.RustCallStatus
	returnValue := callback(&status)
	err := checkCallStatus(converter, status)
	return returnValue, err
}

func checkCallStatus[E any](converter BufReader[*E], status C.RustCallStatus) *E {
	switch status.code {
	case 0:
		return nil
	case 1:
		return LiftFromRustBuffer(converter, GoRustBuffer{inner: status.errorBuf})
	case 2:
		// when the rust code sees a panic, it tries to construct a rustBuffer
		// with the message.  but if that code panics, then it just sends back
		// an empty buffer.
		if status.errorBuf.len > 0 {
			panic(fmt.Errorf("%s", FfiConverterStringINSTANCE.Lift(GoRustBuffer{inner: status.errorBuf})))
		} else {
			panic(fmt.Errorf("Rust panicked while handling Rust panic"))
		}
	default:
		panic(fmt.Errorf("unknown status code: %d", status.code))
	}
}

func checkCallStatusUnknown(status C.RustCallStatus) error {
	switch status.code {
	case 0:
		return nil
	case 1:
		panic(fmt.Errorf("function not returning an error returned an error"))
	case 2:
		// when the rust code sees a panic, it tries to construct a C.RustBuffer
		// with the message.  but if that code panics, then it just sends back
		// an empty buffer.
		if status.errorBuf.len > 0 {
			panic(fmt.Errorf("%s", FfiConverterStringINSTANCE.Lift(GoRustBuffer{
				inner: status.errorBuf,
			})))
		} else {
			panic(fmt.Errorf("Rust panicked while handling Rust panic"))
		}
	default:
		return fmt.Errorf("unknown status code: %d", status.code)
	}
}

func rustCall[U any](callback func(*C.RustCallStatus) U) U {
	returnValue, err := rustCallWithError[error](nil, callback)
	if err != nil {
		panic(err)
	}
	return returnValue
}

type NativeError interface {
	AsError() error
}

func writeInt8(writer io.Writer, value int8) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint8(writer io.Writer, value uint8) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeInt16(writer io.Writer, value int16) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint16(writer io.Writer, value uint16) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeInt32(writer io.Writer, value int32) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint32(writer io.Writer, value uint32) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeInt64(writer io.Writer, value int64) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint64(writer io.Writer, value uint64) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeFloat32(writer io.Writer, value float32) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeFloat64(writer io.Writer, value float64) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func readInt8(reader io.Reader) int8 {
	var result int8
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint8(reader io.Reader) uint8 {
	var result uint8
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readInt16(reader io.Reader) int16 {
	var result int16
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint16(reader io.Reader) uint16 {
	var result uint16
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readInt32(reader io.Reader) int32 {
	var result int32
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint32(reader io.Reader) uint32 {
	var result uint32
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readInt64(reader io.Reader) int64 {
	var result int64
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint64(reader io.Reader) uint64 {
	var result uint64
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readFloat32(reader io.Reader) float32 {
	var result float32
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readFloat64(reader io.Reader) float64 {
	var result float64
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func init() {

	FfiConverterChangeAncestorsTravelerINSTANCE.register()
	FfiConverterContainerIdLikeINSTANCE.register()
	FfiConverterEphemeralSubscriberINSTANCE.register()
	FfiConverterFirstCommitFromPeerCallbackINSTANCE.register()
	FfiConverterLocalEphemeralListenerINSTANCE.register()
	FfiConverterLocalUpdateCallbackINSTANCE.register()
	FfiConverterLoroValueLikeINSTANCE.register()
	FfiConverterOnPopINSTANCE.register()
	FfiConverterOnPushINSTANCE.register()
	FfiConverterPreCommitCallbackINSTANCE.register()
	FfiConverterSubscriberINSTANCE.register()
	FfiConverterUnsubscriberINSTANCE.register()
	uniffiCheckChecksums()
}

func uniffiCheckChecksums() {
	// Get the bindings contract version from our ComponentInterface
	bindingsContractVersion := 26
	// Get the scaffolding contract version by calling the into the dylib
	scaffoldingContractVersion := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.ffi_loro_uniffi_contract_version()
	})
	if bindingsContractVersion != int(scaffoldingContractVersion) {
		// If this happens try cleaning and rebuilding your project
		panic("loro: UniFFI contract version mismatch")
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_func_decode_import_blob_meta()
		})
		if checksum != 2769 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_func_decode_import_blob_meta: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_func_get_version()
		})
		if checksum != 3250 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_func_get_version: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_awareness_apply()
		})
		if checksum != 41900 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_awareness_apply: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_awareness_encode()
		})
		if checksum != 37443 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_awareness_encode: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_awareness_encode_all()
		})
		if checksum != 38982 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_awareness_encode_all: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_awareness_get_all_states()
		})
		if checksum != 30017 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_awareness_get_all_states: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_awareness_get_local_state()
		})
		if checksum != 59706 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_awareness_get_local_state: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_awareness_peer()
		})
		if checksum != 10202 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_awareness_peer: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_awareness_remove_outdated()
		})
		if checksum != 1483 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_awareness_remove_outdated: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_awareness_set_local_state()
		})
		if checksum != 517 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_awareness_set_local_state: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_changeancestorstraveler_travel()
		})
		if checksum != 17239 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_changeancestorstraveler_travel: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_changemodifier_set_message()
		})
		if checksum != 44190 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_changemodifier_set_message: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_changemodifier_set_timestamp()
		})
		if checksum != 6652 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_changemodifier_set_timestamp: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_configure_fork()
		})
		if checksum != 57176 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_configure_fork: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_configure_merge_interval()
		})
		if checksum != 43546 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_configure_merge_interval: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_configure_record_timestamp()
		})
		if checksum != 41033 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_configure_record_timestamp: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_configure_set_merge_interval()
		})
		if checksum != 4893 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_configure_set_merge_interval: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_configure_set_record_timestamp()
		})
		if checksum != 30145 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_configure_set_record_timestamp: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_configure_text_style_config()
		})
		if checksum != 50151 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_configure_text_style_config: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_containeridlike_as_container_id()
		})
		if checksum != 41081 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_containeridlike_as_container_id: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_diffbatch_get_diff()
		})
		if checksum != 42707 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_diffbatch_get_diff: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_diffbatch_push()
		})
		if checksum != 56678 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_diffbatch_push: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_ephemeralstore_apply()
		})
		if checksum != 27412 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_ephemeralstore_apply: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_ephemeralstore_delete()
		})
		if checksum != 37001 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_ephemeralstore_delete: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_ephemeralstore_encode()
		})
		if checksum != 35731 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_ephemeralstore_encode: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_ephemeralstore_encode_all()
		})
		if checksum != 1587 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_ephemeralstore_encode_all: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_ephemeralstore_get()
		})
		if checksum != 37668 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_ephemeralstore_get: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_ephemeralstore_get_all_states()
		})
		if checksum != 11717 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_ephemeralstore_get_all_states: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_ephemeralstore_keys()
		})
		if checksum != 27585 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_ephemeralstore_keys: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_ephemeralstore_remove_outdated()
		})
		if checksum != 35095 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_ephemeralstore_remove_outdated: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_ephemeralstore_set()
		})
		if checksum != 21774 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_ephemeralstore_set: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_ephemeralstore_subscribe()
		})
		if checksum != 54930 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_ephemeralstore_subscribe: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_ephemeralstore_subscribe_local_update()
		})
		if checksum != 54586 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_ephemeralstore_subscribe_local_update: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_ephemeralsubscriber_on_ephemeral_event()
		})
		if checksum != 33183 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_ephemeralsubscriber_on_ephemeral_event: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_firstcommitfrompeercallback_on_first_commit_from_peer()
		})
		if checksum != 51977 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_firstcommitfrompeercallback_on_first_commit_from_peer: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_fractionalindex_to_string()
		})
		if checksum != 57024 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_fractionalindex_to_string: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_frontiers_encode()
		})
		if checksum != 48230 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_frontiers_encode: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_frontiers_eq()
		})
		if checksum != 20207 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_frontiers_eq: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_localephemerallistener_on_ephemeral_update()
		})
		if checksum != 59317 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_localephemerallistener_on_ephemeral_update: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_localupdatecallback_on_local_update()
		})
		if checksum != 21789 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_localupdatecallback_on_local_update: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorocounter_decrement()
		})
		if checksum != 53919 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorocounter_decrement: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorocounter_doc()
		})
		if checksum != 54846 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorocounter_doc: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorocounter_get_attached()
		})
		if checksum != 22021 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorocounter_get_attached: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorocounter_get_value()
		})
		if checksum != 44616 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorocounter_get_value: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorocounter_id()
		})
		if checksum != 31148 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorocounter_id: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorocounter_increment()
		})
		if checksum != 47367 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorocounter_increment: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorocounter_is_attached()
		})
		if checksum != 51768 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorocounter_is_attached: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorocounter_is_deleted()
		})
		if checksum != 12079 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorocounter_is_deleted: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_apply_diff()
		})
		if checksum != 45393 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_apply_diff: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_attach()
		})
		if checksum != 7252 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_attach: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_check_state_correctness_slow()
		})
		if checksum != 43878 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_check_state_correctness_slow: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_checkout()
		})
		if checksum != 415 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_checkout: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_checkout_to_latest()
		})
		if checksum != 2349 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_checkout_to_latest: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_clear_next_commit_options()
		})
		if checksum != 45217 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_clear_next_commit_options: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_cmp_with_frontiers()
		})
		if checksum != 31942 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_cmp_with_frontiers: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_commit()
		})
		if checksum != 53174 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_commit: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_commit_with()
		})
		if checksum != 29999 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_commit_with: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_compact_change_store()
		})
		if checksum != 26224 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_compact_change_store: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_config()
		})
		if checksum != 3400 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_config: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_config_default_text_style()
		})
		if checksum != 15083 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_config_default_text_style: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_config_text_style()
		})
		if checksum != 52393 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_config_text_style: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_delete_root_container()
		})
		if checksum != 40125 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_delete_root_container: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_detach()
		})
		if checksum != 61399 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_detach: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_diff()
		})
		if checksum != 38416 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_diff: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_export_json_in_id_span()
		})
		if checksum != 26608 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_export_json_in_id_span: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_export_json_updates()
		})
		if checksum != 15152 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_export_json_updates: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_export_json_updates_without_peer_compression()
		})
		if checksum != 23184 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_export_json_updates_without_peer_compression: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_export_shallow_snapshot()
		})
		if checksum != 27927 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_export_shallow_snapshot: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_export_snapshot()
		})
		if checksum != 61274 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_export_snapshot: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_export_snapshot_at()
		})
		if checksum != 64602 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_export_snapshot_at: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_export_state_only()
		})
		if checksum != 16747 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_export_state_only: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_export_updates()
		})
		if checksum != 57637 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_export_updates: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_export_updates_in_range()
		})
		if checksum != 22491 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_export_updates_in_range: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_find_id_spans_between()
		})
		if checksum != 1313 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_find_id_spans_between: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_fork()
		})
		if checksum != 45665 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_fork: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_fork_at()
		})
		if checksum != 40377 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_fork_at: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_free_diff_calculator()
		})
		if checksum != 32937 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_free_diff_calculator: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_free_history_cache()
		})
		if checksum != 22144 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_free_history_cache: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_frontiers_to_vv()
		})
		if checksum != 11123 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_frontiers_to_vv: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_get_by_path()
		})
		if checksum != 35945 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_get_by_path: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_get_by_str_path()
		})
		if checksum != 6739 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_get_by_str_path: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_get_change()
		})
		if checksum != 17896 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_get_change: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_get_changed_containers_in()
		})
		if checksum != 52454 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_get_changed_containers_in: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_get_counter()
		})
		if checksum != 12597 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_get_counter: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_get_cursor_pos()
		})
		if checksum != 30480 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_get_cursor_pos: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_get_deep_value()
		})
		if checksum != 3404 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_get_deep_value: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_get_deep_value_with_id()
		})
		if checksum != 49124 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_get_deep_value_with_id: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_get_list()
		})
		if checksum != 9609 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_get_list: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_get_map()
		})
		if checksum != 63137 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_get_map: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_get_movable_list()
		})
		if checksum != 7302 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_get_movable_list: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_get_path_to_container()
		})
		if checksum != 62623 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_get_path_to_container: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_get_pending_txn_len()
		})
		if checksum != 15050 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_get_pending_txn_len: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_get_text()
		})
		if checksum != 56069 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_get_text: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_get_tree()
		})
		if checksum != 54189 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_get_tree: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_get_value()
		})
		if checksum != 29857 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_get_value: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_has_container()
		})
		if checksum != 41856 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_has_container: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_has_history_cache()
		})
		if checksum != 53741 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_has_history_cache: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_import()
		})
		if checksum != 11528 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_import: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_import_batch()
		})
		if checksum != 34010 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_import_batch: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_import_json_updates()
		})
		if checksum != 57379 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_import_json_updates: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_import_with()
		})
		if checksum != 12897 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_import_with: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_is_detached()
		})
		if checksum != 30909 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_is_detached: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_is_shallow()
		})
		if checksum != 53044 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_is_shallow: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_jsonpath()
		})
		if checksum != 15996 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_jsonpath: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_len_changes()
		})
		if checksum != 62401 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_len_changes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_len_ops()
		})
		if checksum != 11644 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_len_ops: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_log_estimate_size()
		})
		if checksum != 19429 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_log_estimate_size: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_minimize_frontiers()
		})
		if checksum != 39579 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_minimize_frontiers: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_oplog_frontiers()
		})
		if checksum != 49043 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_oplog_frontiers: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_oplog_vv()
		})
		if checksum != 56754 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_oplog_vv: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_peer_id()
		})
		if checksum != 35449 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_peer_id: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_revert_to()
		})
		if checksum != 48346 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_revert_to: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_set_change_merge_interval()
		})
		if checksum != 55133 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_set_change_merge_interval: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_set_hide_empty_root_containers()
		})
		if checksum != 34137 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_set_hide_empty_root_containers: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_set_next_commit_message()
		})
		if checksum != 18940 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_set_next_commit_message: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_set_next_commit_options()
		})
		if checksum != 13250 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_set_next_commit_options: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_set_next_commit_origin()
		})
		if checksum != 27549 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_set_next_commit_origin: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_set_next_commit_timestamp()
		})
		if checksum != 30492 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_set_next_commit_timestamp: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_set_peer_id()
		})
		if checksum != 29379 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_set_peer_id: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_set_record_timestamp()
		})
		if checksum != 15945 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_set_record_timestamp: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_shallow_since_vv()
		})
		if checksum != 13449 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_shallow_since_vv: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_state_frontiers()
		})
		if checksum != 17079 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_state_frontiers: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_state_vv()
		})
		if checksum != 1627 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_state_vv: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_subscribe()
		})
		if checksum != 7981 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_subscribe: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_subscribe_first_commit_from_peer()
		})
		if checksum != 45629 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_subscribe_first_commit_from_peer: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_subscribe_local_update()
		})
		if checksum != 58652 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_subscribe_local_update: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_subscribe_pre_commit()
		})
		if checksum != 54832 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_subscribe_pre_commit: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_subscribe_root()
		})
		if checksum != 16564 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_subscribe_root: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_travel_change_ancestors()
		})
		if checksum != 39918 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_travel_change_ancestors: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorodoc_vv_to_frontiers()
		})
		if checksum != 47960 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorodoc_vv_to_frontiers: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorolist_clear()
		})
		if checksum != 61243 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorolist_clear: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorolist_delete()
		})
		if checksum != 40414 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorolist_delete: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorolist_doc()
		})
		if checksum != 4182 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorolist_doc: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorolist_get()
		})
		if checksum != 36174 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorolist_get: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorolist_get_attached()
		})
		if checksum != 5208 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorolist_get_attached: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorolist_get_cursor()
		})
		if checksum != 42636 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorolist_get_cursor: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorolist_get_deep_value()
		})
		if checksum != 9355 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorolist_get_deep_value: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorolist_get_id_at()
		})
		if checksum != 63640 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorolist_get_id_at: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorolist_get_value()
		})
		if checksum != 14384 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorolist_get_value: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorolist_id()
		})
		if checksum != 33887 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorolist_id: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorolist_insert()
		})
		if checksum != 19544 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorolist_insert: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorolist_insert_counter_container()
		})
		if checksum != 41569 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorolist_insert_counter_container: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorolist_insert_list_container()
		})
		if checksum != 42165 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorolist_insert_list_container: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorolist_insert_map_container()
		})
		if checksum != 25622 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorolist_insert_map_container: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorolist_insert_movable_list_container()
		})
		if checksum != 23559 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorolist_insert_movable_list_container: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorolist_insert_text_container()
		})
		if checksum != 26631 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorolist_insert_text_container: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorolist_insert_tree_container()
		})
		if checksum != 15665 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorolist_insert_tree_container: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorolist_is_attached()
		})
		if checksum != 60548 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorolist_is_attached: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorolist_is_deleted()
		})
		if checksum != 44383 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorolist_is_deleted: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorolist_is_empty()
		})
		if checksum != 13469 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorolist_is_empty: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorolist_len()
		})
		if checksum != 22800 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorolist_len: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorolist_pop()
		})
		if checksum != 20748 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorolist_pop: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorolist_push()
		})
		if checksum != 32091 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorolist_push: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorolist_to_vec()
		})
		if checksum != 34199 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorolist_to_vec: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromap_clear()
		})
		if checksum != 22445 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromap_clear: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromap_delete()
		})
		if checksum != 54675 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromap_delete: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromap_doc()
		})
		if checksum != 4684 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromap_doc: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromap_get()
		})
		if checksum != 57695 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromap_get: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromap_get_attached()
		})
		if checksum != 22266 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromap_get_attached: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromap_get_deep_value()
		})
		if checksum != 23748 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromap_get_deep_value: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromap_get_last_editor()
		})
		if checksum != 54864 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromap_get_last_editor: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromap_get_or_create_counter_container()
		})
		if checksum != 34280 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromap_get_or_create_counter_container: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromap_get_or_create_list_container()
		})
		if checksum != 51559 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromap_get_or_create_list_container: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromap_get_or_create_map_container()
		})
		if checksum != 8592 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromap_get_or_create_map_container: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromap_get_or_create_movable_list_container()
		})
		if checksum != 15746 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromap_get_or_create_movable_list_container: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromap_get_or_create_text_container()
		})
		if checksum != 13374 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromap_get_or_create_text_container: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromap_get_or_create_tree_container()
		})
		if checksum != 4760 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromap_get_or_create_tree_container: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromap_get_value()
		})
		if checksum != 7268 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromap_get_value: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromap_id()
		})
		if checksum != 65486 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromap_id: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromap_insert()
		})
		if checksum != 748 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromap_insert: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromap_insert_counter_container()
		})
		if checksum != 5567 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromap_insert_counter_container: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromap_insert_list_container()
		})
		if checksum != 52804 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromap_insert_list_container: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromap_insert_map_container()
		})
		if checksum != 36523 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromap_insert_map_container: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromap_insert_movable_list_container()
		})
		if checksum != 21076 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromap_insert_movable_list_container: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromap_insert_text_container()
		})
		if checksum != 50348 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromap_insert_text_container: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromap_insert_tree_container()
		})
		if checksum != 59771 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromap_insert_tree_container: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromap_is_attached()
		})
		if checksum != 6283 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromap_is_attached: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromap_is_deleted()
		})
		if checksum != 20980 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromap_is_deleted: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromap_is_empty()
		})
		if checksum != 38768 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromap_is_empty: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromap_keys()
		})
		if checksum != 11621 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromap_keys: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromap_len()
		})
		if checksum != 38088 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromap_len: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromap_values()
		})
		if checksum != 46629 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromap_values: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromovablelist_clear()
		})
		if checksum != 17252 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromovablelist_clear: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromovablelist_delete()
		})
		if checksum != 51786 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromovablelist_delete: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromovablelist_doc()
		})
		if checksum != 13729 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromovablelist_doc: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromovablelist_get()
		})
		if checksum != 10599 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromovablelist_get: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromovablelist_get_attached()
		})
		if checksum != 53503 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromovablelist_get_attached: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromovablelist_get_creator_at()
		})
		if checksum != 21542 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromovablelist_get_creator_at: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromovablelist_get_cursor()
		})
		if checksum != 118 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromovablelist_get_cursor: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromovablelist_get_deep_value()
		})
		if checksum != 18542 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromovablelist_get_deep_value: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromovablelist_get_last_editor_at()
		})
		if checksum != 8998 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromovablelist_get_last_editor_at: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromovablelist_get_last_mover_at()
		})
		if checksum != 26603 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromovablelist_get_last_mover_at: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromovablelist_get_value()
		})
		if checksum != 50843 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromovablelist_get_value: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromovablelist_id()
		})
		if checksum != 9803 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromovablelist_id: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromovablelist_insert()
		})
		if checksum != 28537 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromovablelist_insert: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromovablelist_insert_counter_container()
		})
		if checksum != 56222 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromovablelist_insert_counter_container: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromovablelist_insert_list_container()
		})
		if checksum != 47190 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromovablelist_insert_list_container: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromovablelist_insert_map_container()
		})
		if checksum != 57810 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromovablelist_insert_map_container: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromovablelist_insert_movable_list_container()
		})
		if checksum != 6019 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromovablelist_insert_movable_list_container: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromovablelist_insert_text_container()
		})
		if checksum != 48945 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromovablelist_insert_text_container: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromovablelist_insert_tree_container()
		})
		if checksum != 33670 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromovablelist_insert_tree_container: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromovablelist_is_attached()
		})
		if checksum != 50724 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromovablelist_is_attached: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromovablelist_is_deleted()
		})
		if checksum != 37438 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromovablelist_is_deleted: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromovablelist_is_empty()
		})
		if checksum != 44651 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromovablelist_is_empty: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromovablelist_len()
		})
		if checksum != 28945 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromovablelist_len: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromovablelist_mov()
		})
		if checksum != 8301 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromovablelist_mov: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromovablelist_pop()
		})
		if checksum != 52086 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromovablelist_pop: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromovablelist_push()
		})
		if checksum != 2828 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromovablelist_push: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromovablelist_set()
		})
		if checksum != 27054 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromovablelist_set: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromovablelist_set_counter_container()
		})
		if checksum != 1414 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromovablelist_set_counter_container: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromovablelist_set_list_container()
		})
		if checksum != 20393 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromovablelist_set_list_container: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromovablelist_set_map_container()
		})
		if checksum != 20297 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromovablelist_set_map_container: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromovablelist_set_movable_list_container()
		})
		if checksum != 52254 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromovablelist_set_movable_list_container: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromovablelist_set_text_container()
		})
		if checksum != 31935 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromovablelist_set_text_container: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromovablelist_set_tree_container()
		})
		if checksum != 8298 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromovablelist_set_tree_container: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_loromovablelist_to_vec()
		})
		if checksum != 28826 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_loromovablelist_to_vec: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotext_apply_delta()
		})
		if checksum != 32084 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotext_apply_delta: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotext_delete()
		})
		if checksum != 47933 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotext_delete: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotext_delete_utf8()
		})
		if checksum != 44384 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotext_delete_utf8: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotext_doc()
		})
		if checksum != 41195 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotext_doc: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotext_get_attached()
		})
		if checksum != 16919 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotext_get_attached: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotext_get_cursor()
		})
		if checksum != 57876 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotext_get_cursor: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotext_get_editor_at_unicode_pos()
		})
		if checksum != 24596 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotext_get_editor_at_unicode_pos: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotext_get_richtext_value()
		})
		if checksum != 45999 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotext_get_richtext_value: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotext_id()
		})
		if checksum != 30925 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotext_id: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotext_insert()
		})
		if checksum != 10847 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotext_insert: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotext_insert_utf8()
		})
		if checksum != 8484 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotext_insert_utf8: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotext_is_attached()
		})
		if checksum != 45378 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotext_is_attached: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotext_is_deleted()
		})
		if checksum != 31871 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotext_is_deleted: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotext_is_empty()
		})
		if checksum != 7961 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotext_is_empty: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotext_len_unicode()
		})
		if checksum != 46650 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotext_len_unicode: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotext_len_utf16()
		})
		if checksum != 18865 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotext_len_utf16: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotext_len_utf8()
		})
		if checksum != 29025 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotext_len_utf8: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotext_mark()
		})
		if checksum != 42690 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotext_mark: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotext_push_str()
		})
		if checksum != 2374 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotext_push_str: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotext_slice()
		})
		if checksum != 43152 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotext_slice: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotext_splice()
		})
		if checksum != 30467 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotext_splice: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotext_to_delta()
		})
		if checksum != 15868 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotext_to_delta: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotext_to_string()
		})
		if checksum != 63765 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotext_to_string: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotext_unmark()
		})
		if checksum != 14351 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotext_unmark: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotext_update()
		})
		if checksum != 16538 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotext_update: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotext_update_by_line()
		})
		if checksum != 29301 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotext_update_by_line: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotree_children()
		})
		if checksum != 4750 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotree_children: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotree_children_num()
		})
		if checksum != 50997 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotree_children_num: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotree_contains()
		})
		if checksum != 62174 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotree_contains: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotree_create()
		})
		if checksum != 55490 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotree_create: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotree_create_at()
		})
		if checksum != 64751 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotree_create_at: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotree_delete()
		})
		if checksum != 36355 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotree_delete: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotree_disable_fractional_index()
		})
		if checksum != 52853 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotree_disable_fractional_index: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotree_doc()
		})
		if checksum != 23287 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotree_doc: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotree_enable_fractional_index()
		})
		if checksum != 39633 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotree_enable_fractional_index: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotree_fractional_index()
		})
		if checksum != 51036 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotree_fractional_index: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotree_get_attached()
		})
		if checksum != 57142 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotree_get_attached: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotree_get_last_move_id()
		})
		if checksum != 12557 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotree_get_last_move_id: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotree_get_meta()
		})
		if checksum != 3068 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotree_get_meta: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotree_get_value()
		})
		if checksum != 44704 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotree_get_value: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotree_get_value_with_meta()
		})
		if checksum != 7497 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotree_get_value_with_meta: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotree_id()
		})
		if checksum != 4862 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotree_id: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotree_is_attached()
		})
		if checksum != 37303 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotree_is_attached: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotree_is_deleted()
		})
		if checksum != 8644 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotree_is_deleted: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotree_is_fractional_index_enabled()
		})
		if checksum != 19364 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotree_is_fractional_index_enabled: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotree_is_node_deleted()
		})
		if checksum != 7339 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotree_is_node_deleted: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotree_mov()
		})
		if checksum != 33288 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotree_mov: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotree_mov_after()
		})
		if checksum != 48871 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotree_mov_after: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotree_mov_before()
		})
		if checksum != 39654 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotree_mov_before: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotree_mov_to()
		})
		if checksum != 21629 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotree_mov_to: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotree_nodes()
		})
		if checksum != 31738 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotree_nodes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotree_parent()
		})
		if checksum != 6903 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotree_parent: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorotree_roots()
		})
		if checksum != 60881 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorotree_roots: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorounknown_id()
		})
		if checksum != 65156 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorounknown_id: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_lorovaluelike_as_loro_value()
		})
		if checksum != 23668 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_lorovaluelike_as_loro_value: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_onpop_on_pop()
		})
		if checksum != 39438 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_onpop_on_pop: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_onpush_on_push()
		})
		if checksum != 46111 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_onpush_on_push: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_precommitcallback_on_pre_commit()
		})
		if checksum != 58911 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_precommitcallback_on_pre_commit: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_styleconfigmap_get()
		})
		if checksum != 25442 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_styleconfigmap_get: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_styleconfigmap_insert()
		})
		if checksum != 49128 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_styleconfigmap_insert: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_subscriber_on_diff()
		})
		if checksum != 462 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_subscriber_on_diff: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_subscription_detach()
		})
		if checksum != 64699 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_subscription_detach: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_subscription_unsubscribe()
		})
		if checksum != 24473 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_subscription_unsubscribe: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_undomanager_add_exclude_origin_prefix()
		})
		if checksum != 61306 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_undomanager_add_exclude_origin_prefix: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_undomanager_can_redo()
		})
		if checksum != 61543 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_undomanager_can_redo: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_undomanager_can_undo()
		})
		if checksum != 51532 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_undomanager_can_undo: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_undomanager_record_new_checkpoint()
		})
		if checksum != 28438 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_undomanager_record_new_checkpoint: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_undomanager_redo()
		})
		if checksum != 54874 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_undomanager_redo: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_undomanager_set_max_undo_steps()
		})
		if checksum != 43243 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_undomanager_set_max_undo_steps: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_undomanager_set_merge_interval()
		})
		if checksum != 13688 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_undomanager_set_merge_interval: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_undomanager_set_on_pop()
		})
		if checksum != 4141 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_undomanager_set_on_pop: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_undomanager_set_on_push()
		})
		if checksum != 31009 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_undomanager_set_on_push: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_undomanager_undo()
		})
		if checksum != 14430 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_undomanager_undo: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_unsubscriber_on_unsubscribe()
		})
		if checksum != 17877 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_unsubscriber_on_unsubscribe: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_valueorcontainer_as_container()
		})
		if checksum != 61163 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_valueorcontainer_as_container: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_valueorcontainer_as_loro_counter()
		})
		if checksum != 51072 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_valueorcontainer_as_loro_counter: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_valueorcontainer_as_loro_list()
		})
		if checksum != 16144 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_valueorcontainer_as_loro_list: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_valueorcontainer_as_loro_map()
		})
		if checksum != 62391 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_valueorcontainer_as_loro_map: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_valueorcontainer_as_loro_movable_list()
		})
		if checksum != 49359 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_valueorcontainer_as_loro_movable_list: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_valueorcontainer_as_loro_text()
		})
		if checksum != 8015 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_valueorcontainer_as_loro_text: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_valueorcontainer_as_loro_tree()
		})
		if checksum != 39545 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_valueorcontainer_as_loro_tree: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_valueorcontainer_as_loro_unknown()
		})
		if checksum != 9911 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_valueorcontainer_as_loro_unknown: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_valueorcontainer_as_value()
		})
		if checksum != 9638 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_valueorcontainer_as_value: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_valueorcontainer_container_type()
		})
		if checksum != 56498 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_valueorcontainer_container_type: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_valueorcontainer_is_container()
		})
		if checksum != 16329 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_valueorcontainer_is_container: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_valueorcontainer_is_value()
		})
		if checksum != 13911 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_valueorcontainer_is_value: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_versionvector_diff()
		})
		if checksum != 29529 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_versionvector_diff: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_versionvector_encode()
		})
		if checksum != 64665 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_versionvector_encode: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_versionvector_eq()
		})
		if checksum != 30406 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_versionvector_eq: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_versionvector_extend_to_include_vv()
		})
		if checksum != 28076 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_versionvector_extend_to_include_vv: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_versionvector_get_last()
		})
		if checksum != 45434 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_versionvector_get_last: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_versionvector_get_missing_span()
		})
		if checksum != 2797 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_versionvector_get_missing_span: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_versionvector_includes_id()
		})
		if checksum != 50408 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_versionvector_includes_id: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_versionvector_includes_vv()
		})
		if checksum != 23089 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_versionvector_includes_vv: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_versionvector_intersect_span()
		})
		if checksum != 54748 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_versionvector_intersect_span: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_versionvector_merge()
		})
		if checksum != 23694 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_versionvector_merge: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_versionvector_partial_cmp()
		})
		if checksum != 27570 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_versionvector_partial_cmp: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_versionvector_set_end()
		})
		if checksum != 27313 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_versionvector_set_end: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_method_versionvector_set_last()
		})
		if checksum != 40968 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_method_versionvector_set_last: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_constructor_awareness_new()
		})
		if checksum != 33037 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_constructor_awareness_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_constructor_cursor_new()
		})
		if checksum != 11721 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_constructor_cursor_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_constructor_diffbatch_new()
		})
		if checksum != 62583 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_constructor_diffbatch_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_constructor_ephemeralstore_new()
		})
		if checksum != 60190 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_constructor_ephemeralstore_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_constructor_fractionalindex_from_bytes()
		})
		if checksum != 35415 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_constructor_fractionalindex_from_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_constructor_fractionalindex_from_hex_string()
		})
		if checksum != 10737 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_constructor_fractionalindex_from_hex_string: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_constructor_frontiers_decode()
		})
		if checksum != 27043 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_constructor_frontiers_decode: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_constructor_frontiers_from_id()
		})
		if checksum != 60928 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_constructor_frontiers_from_id: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_constructor_frontiers_from_ids()
		})
		if checksum != 61832 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_constructor_frontiers_from_ids: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_constructor_frontiers_new()
		})
		if checksum != 5172 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_constructor_frontiers_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_constructor_lorocounter_new()
		})
		if checksum != 44867 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_constructor_lorocounter_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_constructor_lorodoc_new()
		})
		if checksum != 54129 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_constructor_lorodoc_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_constructor_lorolist_new()
		})
		if checksum != 2861 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_constructor_lorolist_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_constructor_loromap_new()
		})
		if checksum != 63806 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_constructor_loromap_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_constructor_loromovablelist_new()
		})
		if checksum != 32944 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_constructor_loromovablelist_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_constructor_lorotext_new()
		})
		if checksum != 33191 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_constructor_lorotext_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_constructor_lorotree_new()
		})
		if checksum != 42150 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_constructor_lorotree_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_constructor_styleconfigmap_default_rich_text_config()
		})
		if checksum != 46944 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_constructor_styleconfigmap_default_rich_text_config: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_constructor_styleconfigmap_new()
		})
		if checksum != 23831 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_constructor_styleconfigmap_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_constructor_undomanager_new()
		})
		if checksum != 35328 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_constructor_undomanager_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_constructor_versionvector_decode()
		})
		if checksum != 19639 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_constructor_versionvector_decode: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_loro_checksum_constructor_versionvector_new()
		})
		if checksum != 31126 {
			// If this happens try cleaning and rebuilding your project
			panic("loro: uniffi_loro_checksum_constructor_versionvector_new: UniFFI API checksum mismatch")
		}
	}
}

type FfiConverterUint8 struct{}

var FfiConverterUint8INSTANCE = FfiConverterUint8{}

func (FfiConverterUint8) Lower(value uint8) C.uint8_t {
	return C.uint8_t(value)
}

func (FfiConverterUint8) Write(writer io.Writer, value uint8) {
	writeUint8(writer, value)
}

func (FfiConverterUint8) Lift(value C.uint8_t) uint8 {
	return uint8(value)
}

func (FfiConverterUint8) Read(reader io.Reader) uint8 {
	return readUint8(reader)
}

type FfiDestroyerUint8 struct{}

func (FfiDestroyerUint8) Destroy(_ uint8) {}

type FfiConverterUint32 struct{}

var FfiConverterUint32INSTANCE = FfiConverterUint32{}

func (FfiConverterUint32) Lower(value uint32) C.uint32_t {
	return C.uint32_t(value)
}

func (FfiConverterUint32) Write(writer io.Writer, value uint32) {
	writeUint32(writer, value)
}

func (FfiConverterUint32) Lift(value C.uint32_t) uint32 {
	return uint32(value)
}

func (FfiConverterUint32) Read(reader io.Reader) uint32 {
	return readUint32(reader)
}

type FfiDestroyerUint32 struct{}

func (FfiDestroyerUint32) Destroy(_ uint32) {}

type FfiConverterInt32 struct{}

var FfiConverterInt32INSTANCE = FfiConverterInt32{}

func (FfiConverterInt32) Lower(value int32) C.int32_t {
	return C.int32_t(value)
}

func (FfiConverterInt32) Write(writer io.Writer, value int32) {
	writeInt32(writer, value)
}

func (FfiConverterInt32) Lift(value C.int32_t) int32 {
	return int32(value)
}

func (FfiConverterInt32) Read(reader io.Reader) int32 {
	return readInt32(reader)
}

type FfiDestroyerInt32 struct{}

func (FfiDestroyerInt32) Destroy(_ int32) {}

type FfiConverterUint64 struct{}

var FfiConverterUint64INSTANCE = FfiConverterUint64{}

func (FfiConverterUint64) Lower(value uint64) C.uint64_t {
	return C.uint64_t(value)
}

func (FfiConverterUint64) Write(writer io.Writer, value uint64) {
	writeUint64(writer, value)
}

func (FfiConverterUint64) Lift(value C.uint64_t) uint64 {
	return uint64(value)
}

func (FfiConverterUint64) Read(reader io.Reader) uint64 {
	return readUint64(reader)
}

type FfiDestroyerUint64 struct{}

func (FfiDestroyerUint64) Destroy(_ uint64) {}

type FfiConverterInt64 struct{}

var FfiConverterInt64INSTANCE = FfiConverterInt64{}

func (FfiConverterInt64) Lower(value int64) C.int64_t {
	return C.int64_t(value)
}

func (FfiConverterInt64) Write(writer io.Writer, value int64) {
	writeInt64(writer, value)
}

func (FfiConverterInt64) Lift(value C.int64_t) int64 {
	return int64(value)
}

func (FfiConverterInt64) Read(reader io.Reader) int64 {
	return readInt64(reader)
}

type FfiDestroyerInt64 struct{}

func (FfiDestroyerInt64) Destroy(_ int64) {}

type FfiConverterFloat64 struct{}

var FfiConverterFloat64INSTANCE = FfiConverterFloat64{}

func (FfiConverterFloat64) Lower(value float64) C.double {
	return C.double(value)
}

func (FfiConverterFloat64) Write(writer io.Writer, value float64) {
	writeFloat64(writer, value)
}

func (FfiConverterFloat64) Lift(value C.double) float64 {
	return float64(value)
}

func (FfiConverterFloat64) Read(reader io.Reader) float64 {
	return readFloat64(reader)
}

type FfiDestroyerFloat64 struct{}

func (FfiDestroyerFloat64) Destroy(_ float64) {}

type FfiConverterBool struct{}

var FfiConverterBoolINSTANCE = FfiConverterBool{}

func (FfiConverterBool) Lower(value bool) C.int8_t {
	if value {
		return C.int8_t(1)
	}
	return C.int8_t(0)
}

func (FfiConverterBool) Write(writer io.Writer, value bool) {
	if value {
		writeInt8(writer, 1)
	} else {
		writeInt8(writer, 0)
	}
}

func (FfiConverterBool) Lift(value C.int8_t) bool {
	return value != 0
}

func (FfiConverterBool) Read(reader io.Reader) bool {
	return readInt8(reader) != 0
}

type FfiDestroyerBool struct{}

func (FfiDestroyerBool) Destroy(_ bool) {}

type FfiConverterString struct{}

var FfiConverterStringINSTANCE = FfiConverterString{}

func (FfiConverterString) Lift(rb RustBufferI) string {
	defer rb.Free()
	reader := rb.AsReader()
	b, err := io.ReadAll(reader)
	if err != nil {
		panic(fmt.Errorf("reading reader: %w", err))
	}
	return string(b)
}

func (FfiConverterString) Read(reader io.Reader) string {
	length := readInt32(reader)
	buffer := make([]byte, length)
	read_length, err := reader.Read(buffer)
	if err != nil && err != io.EOF {
		panic(err)
	}
	if read_length != int(length) {
		panic(fmt.Errorf("bad read length when reading string, expected %d, read %d", length, read_length))
	}
	return string(buffer)
}

func (FfiConverterString) Lower(value string) C.RustBuffer {
	return stringToRustBuffer(value)
}

func (FfiConverterString) Write(writer io.Writer, value string) {
	if len(value) > math.MaxInt32 {
		panic("String is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	write_length, err := io.WriteString(writer, value)
	if err != nil {
		panic(err)
	}
	if write_length != len(value) {
		panic(fmt.Errorf("bad write length when writing string, expected %d, written %d", len(value), write_length))
	}
}

type FfiDestroyerString struct{}

func (FfiDestroyerString) Destroy(_ string) {}

type FfiConverterBytes struct{}

var FfiConverterBytesINSTANCE = FfiConverterBytes{}

func (c FfiConverterBytes) Lower(value []byte) C.RustBuffer {
	return LowerIntoRustBuffer[[]byte](c, value)
}

func (c FfiConverterBytes) Write(writer io.Writer, value []byte) {
	if len(value) > math.MaxInt32 {
		panic("[]byte is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	write_length, err := writer.Write(value)
	if err != nil {
		panic(err)
	}
	if write_length != len(value) {
		panic(fmt.Errorf("bad write length when writing []byte, expected %d, written %d", len(value), write_length))
	}
}

func (c FfiConverterBytes) Lift(rb RustBufferI) []byte {
	return LiftFromRustBuffer[[]byte](c, rb)
}

func (c FfiConverterBytes) Read(reader io.Reader) []byte {
	length := readInt32(reader)
	buffer := make([]byte, length)
	read_length, err := reader.Read(buffer)
	if err != nil && err != io.EOF {
		panic(err)
	}
	if read_length != int(length) {
		panic(fmt.Errorf("bad read length when reading []byte, expected %d, read %d", length, read_length))
	}
	return buffer
}

type FfiDestroyerBytes struct{}

func (FfiDestroyerBytes) Destroy(_ []byte) {}

// Below is an implementation of synchronization requirements outlined in the link.
// https://github.com/mozilla/uniffi-rs/blob/0dc031132d9493ca812c3af6e7dd60ad2ea95bf0/uniffi_bindgen/src/bindings/kotlin/templates/ObjectRuntime.kt#L31

type FfiObject struct {
	pointer       unsafe.Pointer
	callCounter   atomic.Int64
	cloneFunction func(unsafe.Pointer, *C.RustCallStatus) unsafe.Pointer
	freeFunction  func(unsafe.Pointer, *C.RustCallStatus)
	destroyed     atomic.Bool
}

func newFfiObject(
	pointer unsafe.Pointer,
	cloneFunction func(unsafe.Pointer, *C.RustCallStatus) unsafe.Pointer,
	freeFunction func(unsafe.Pointer, *C.RustCallStatus),
) FfiObject {
	return FfiObject{
		pointer:       pointer,
		cloneFunction: cloneFunction,
		freeFunction:  freeFunction,
	}
}

func (ffiObject *FfiObject) incrementPointer(debugName string) unsafe.Pointer {
	for {
		counter := ffiObject.callCounter.Load()
		if counter <= -1 {
			panic(fmt.Errorf("%v object has already been destroyed", debugName))
		}
		if counter == math.MaxInt64 {
			panic(fmt.Errorf("%v object call counter would overflow", debugName))
		}
		if ffiObject.callCounter.CompareAndSwap(counter, counter+1) {
			break
		}
	}

	return rustCall(func(status *C.RustCallStatus) unsafe.Pointer {
		return ffiObject.cloneFunction(ffiObject.pointer, status)
	})
}

func (ffiObject *FfiObject) decrementPointer() {
	if ffiObject.callCounter.Add(-1) == -1 {
		ffiObject.freeRustArcPtr()
	}
}

func (ffiObject *FfiObject) destroy() {
	if ffiObject.destroyed.CompareAndSwap(false, true) {
		if ffiObject.callCounter.Add(-1) == -1 {
			ffiObject.freeRustArcPtr()
		}
	}
}

func (ffiObject *FfiObject) freeRustArcPtr() {
	rustCall(func(status *C.RustCallStatus) int32 {
		ffiObject.freeFunction(ffiObject.pointer, status)
		return 0
	})
}

// Deprecated, use `EphemeralStore` instead.
type AwarenessInterface interface {
	Apply(encodedPeersInfo []byte) AwarenessPeerUpdate
	Encode(peers []uint64) []byte
	EncodeAll() []byte
	GetAllStates() map[uint64]PeerInfo
	GetLocalState() *LoroValue
	Peer() uint64
	RemoveOutdated() []uint64
	SetLocalState(value LoroValueLike)
}

// Deprecated, use `EphemeralStore` instead.
type Awareness struct {
	ffiObject FfiObject
}

func NewAwareness(peer uint64, timeout int64) *Awareness {
	return FfiConverterAwarenessINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_constructor_awareness_new(FfiConverterUint64INSTANCE.Lower(peer), FfiConverterInt64INSTANCE.Lower(timeout), _uniffiStatus)
	}))
}

func (_self *Awareness) Apply(encodedPeersInfo []byte) AwarenessPeerUpdate {
	_pointer := _self.ffiObject.incrementPointer("*Awareness")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterAwarenessPeerUpdateINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_awareness_apply(
				_pointer, FfiConverterBytesINSTANCE.Lower(encodedPeersInfo), _uniffiStatus),
		}
	}))
}

func (_self *Awareness) Encode(peers []uint64) []byte {
	_pointer := _self.ffiObject.incrementPointer("*Awareness")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBytesINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_awareness_encode(
				_pointer, FfiConverterSequenceUint64INSTANCE.Lower(peers), _uniffiStatus),
		}
	}))
}

func (_self *Awareness) EncodeAll() []byte {
	_pointer := _self.ffiObject.incrementPointer("*Awareness")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBytesINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_awareness_encode_all(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Awareness) GetAllStates() map[uint64]PeerInfo {
	_pointer := _self.ffiObject.incrementPointer("*Awareness")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterMapUint64PeerInfoINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_awareness_get_all_states(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Awareness) GetLocalState() *LoroValue {
	_pointer := _self.ffiObject.incrementPointer("*Awareness")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalLoroValueINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_awareness_get_local_state(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Awareness) Peer() uint64 {
	_pointer := _self.ffiObject.incrementPointer("*Awareness")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint64_t {
		return C.uniffi_loro_fn_method_awareness_peer(
			_pointer, _uniffiStatus)
	}))
}

func (_self *Awareness) RemoveOutdated() []uint64 {
	_pointer := _self.ffiObject.incrementPointer("*Awareness")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_awareness_remove_outdated(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Awareness) SetLocalState(value LoroValueLike) {
	_pointer := _self.ffiObject.incrementPointer("*Awareness")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_awareness_set_local_state(
			_pointer, FfiConverterLoroValueLikeINSTANCE.Lower(value), _uniffiStatus)
		return false
	})
}
func (object *Awareness) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterAwareness struct{}

var FfiConverterAwarenessINSTANCE = FfiConverterAwareness{}

func (c FfiConverterAwareness) Lift(pointer unsafe.Pointer) *Awareness {
	result := &Awareness{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_loro_fn_clone_awareness(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_loro_fn_free_awareness(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Awareness).Destroy)
	return result
}

func (c FfiConverterAwareness) Read(reader io.Reader) *Awareness {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterAwareness) Lower(value *Awareness) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Awareness")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterAwareness) Write(writer io.Writer, value *Awareness) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerAwareness struct{}

func (_ FfiDestroyerAwareness) Destroy(value *Awareness) {
	value.Destroy()
}

type ChangeAncestorsTraveler interface {
	Travel(change ChangeMeta) bool
}
type ChangeAncestorsTravelerImpl struct {
	ffiObject FfiObject
}

func (_self *ChangeAncestorsTravelerImpl) Travel(change ChangeMeta) bool {
	_pointer := _self.ffiObject.incrementPointer("ChangeAncestorsTraveler")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_loro_fn_method_changeancestorstraveler_travel(
			_pointer, FfiConverterChangeMetaINSTANCE.Lower(change), _uniffiStatus)
	}))
}
func (object *ChangeAncestorsTravelerImpl) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterChangeAncestorsTraveler struct {
	handleMap *concurrentHandleMap[ChangeAncestorsTraveler]
}

var FfiConverterChangeAncestorsTravelerINSTANCE = FfiConverterChangeAncestorsTraveler{
	handleMap: newConcurrentHandleMap[ChangeAncestorsTraveler](),
}

func (c FfiConverterChangeAncestorsTraveler) Lift(pointer unsafe.Pointer) ChangeAncestorsTraveler {
	result := &ChangeAncestorsTravelerImpl{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_loro_fn_clone_changeancestorstraveler(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_loro_fn_free_changeancestorstraveler(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*ChangeAncestorsTravelerImpl).Destroy)
	return result
}

func (c FfiConverterChangeAncestorsTraveler) Read(reader io.Reader) ChangeAncestorsTraveler {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterChangeAncestorsTraveler) Lower(value ChangeAncestorsTraveler) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := unsafe.Pointer(uintptr(c.handleMap.insert(value)))
	return pointer

}

func (c FfiConverterChangeAncestorsTraveler) Write(writer io.Writer, value ChangeAncestorsTraveler) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerChangeAncestorsTraveler struct{}

func (_ FfiDestroyerChangeAncestorsTraveler) Destroy(value ChangeAncestorsTraveler) {
	if val, ok := value.(*ChangeAncestorsTravelerImpl); ok {
		val.Destroy()
	} else {
		panic("Expected *ChangeAncestorsTravelerImpl")
	}
}

type uniffiCallbackResult C.int8_t

const (
	uniffiIdxCallbackFree               uniffiCallbackResult = 0
	uniffiCallbackResultSuccess         uniffiCallbackResult = 0
	uniffiCallbackResultError           uniffiCallbackResult = 1
	uniffiCallbackUnexpectedResultError uniffiCallbackResult = 2
	uniffiCallbackCancelled             uniffiCallbackResult = 3
)

type concurrentHandleMap[T any] struct {
	handles       map[uint64]T
	currentHandle uint64
	lock          sync.RWMutex
}

func newConcurrentHandleMap[T any]() *concurrentHandleMap[T] {
	return &concurrentHandleMap[T]{
		handles: map[uint64]T{},
	}
}

func (cm *concurrentHandleMap[T]) insert(obj T) uint64 {
	cm.lock.Lock()
	defer cm.lock.Unlock()

	cm.currentHandle = cm.currentHandle + 1
	cm.handles[cm.currentHandle] = obj
	return cm.currentHandle
}

func (cm *concurrentHandleMap[T]) remove(handle uint64) {
	cm.lock.Lock()
	defer cm.lock.Unlock()

	delete(cm.handles, handle)
}

func (cm *concurrentHandleMap[T]) tryGet(handle uint64) (T, bool) {
	cm.lock.RLock()
	defer cm.lock.RUnlock()

	val, ok := cm.handles[handle]
	return val, ok
}

//export loro_cgo_dispatchCallbackInterfaceChangeAncestorsTravelerMethod0
func loro_cgo_dispatchCallbackInterfaceChangeAncestorsTravelerMethod0(uniffiHandle C.uint64_t, change C.RustBuffer, uniffiOutReturn *C.int8_t, callStatus *C.RustCallStatus) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterChangeAncestorsTravelerINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	res :=
		uniffiObj.Travel(
			FfiConverterChangeMetaINSTANCE.Lift(GoRustBuffer{
				inner: change,
			}),
		)

	*uniffiOutReturn = FfiConverterBoolINSTANCE.Lower(res)
}

var UniffiVTableCallbackInterfaceChangeAncestorsTravelerINSTANCE = C.UniffiVTableCallbackInterfaceChangeAncestorsTraveler{
	travel: (C.UniffiCallbackInterfaceChangeAncestorsTravelerMethod0)(C.loro_cgo_dispatchCallbackInterfaceChangeAncestorsTravelerMethod0),

	uniffiFree: (C.UniffiCallbackInterfaceFree)(C.loro_cgo_dispatchCallbackInterfaceChangeAncestorsTravelerFree),
}

//export loro_cgo_dispatchCallbackInterfaceChangeAncestorsTravelerFree
func loro_cgo_dispatchCallbackInterfaceChangeAncestorsTravelerFree(handle C.uint64_t) {
	FfiConverterChangeAncestorsTravelerINSTANCE.handleMap.remove(uint64(handle))
}

func (c FfiConverterChangeAncestorsTraveler) register() {
	C.uniffi_loro_fn_init_callback_vtable_changeancestorstraveler(&UniffiVTableCallbackInterfaceChangeAncestorsTravelerINSTANCE)
}

type ChangeModifierInterface interface {
	SetMessage(msg string)
	SetTimestamp(timestamp int64)
}
type ChangeModifier struct {
	ffiObject FfiObject
}

func (_self *ChangeModifier) SetMessage(msg string) {
	_pointer := _self.ffiObject.incrementPointer("*ChangeModifier")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_changemodifier_set_message(
			_pointer, FfiConverterStringINSTANCE.Lower(msg), _uniffiStatus)
		return false
	})
}

func (_self *ChangeModifier) SetTimestamp(timestamp int64) {
	_pointer := _self.ffiObject.incrementPointer("*ChangeModifier")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_changemodifier_set_timestamp(
			_pointer, FfiConverterInt64INSTANCE.Lower(timestamp), _uniffiStatus)
		return false
	})
}
func (object *ChangeModifier) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterChangeModifier struct{}

var FfiConverterChangeModifierINSTANCE = FfiConverterChangeModifier{}

func (c FfiConverterChangeModifier) Lift(pointer unsafe.Pointer) *ChangeModifier {
	result := &ChangeModifier{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_loro_fn_clone_changemodifier(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_loro_fn_free_changemodifier(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*ChangeModifier).Destroy)
	return result
}

func (c FfiConverterChangeModifier) Read(reader io.Reader) *ChangeModifier {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterChangeModifier) Lower(value *ChangeModifier) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*ChangeModifier")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterChangeModifier) Write(writer io.Writer, value *ChangeModifier) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerChangeModifier struct{}

func (_ FfiDestroyerChangeModifier) Destroy(value *ChangeModifier) {
	value.Destroy()
}

type ConfigureInterface interface {
	Fork() *Configure
	MergeInterval() int64
	RecordTimestamp() bool
	SetMergeInterval(interval int64)
	SetRecordTimestamp(record bool)
	TextStyleConfig() *StyleConfigMap
}
type Configure struct {
	ffiObject FfiObject
}

func (_self *Configure) Fork() *Configure {
	_pointer := _self.ffiObject.incrementPointer("*Configure")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterConfigureINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_configure_fork(
			_pointer, _uniffiStatus)
	}))
}

func (_self *Configure) MergeInterval() int64 {
	_pointer := _self.ffiObject.incrementPointer("*Configure")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterInt64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int64_t {
		return C.uniffi_loro_fn_method_configure_merge_interval(
			_pointer, _uniffiStatus)
	}))
}

func (_self *Configure) RecordTimestamp() bool {
	_pointer := _self.ffiObject.incrementPointer("*Configure")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_loro_fn_method_configure_record_timestamp(
			_pointer, _uniffiStatus)
	}))
}

func (_self *Configure) SetMergeInterval(interval int64) {
	_pointer := _self.ffiObject.incrementPointer("*Configure")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_configure_set_merge_interval(
			_pointer, FfiConverterInt64INSTANCE.Lower(interval), _uniffiStatus)
		return false
	})
}

func (_self *Configure) SetRecordTimestamp(record bool) {
	_pointer := _self.ffiObject.incrementPointer("*Configure")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_configure_set_record_timestamp(
			_pointer, FfiConverterBoolINSTANCE.Lower(record), _uniffiStatus)
		return false
	})
}

func (_self *Configure) TextStyleConfig() *StyleConfigMap {
	_pointer := _self.ffiObject.incrementPointer("*Configure")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStyleConfigMapINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_configure_text_style_config(
			_pointer, _uniffiStatus)
	}))
}
func (object *Configure) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterConfigure struct{}

var FfiConverterConfigureINSTANCE = FfiConverterConfigure{}

func (c FfiConverterConfigure) Lift(pointer unsafe.Pointer) *Configure {
	result := &Configure{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_loro_fn_clone_configure(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_loro_fn_free_configure(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Configure).Destroy)
	return result
}

func (c FfiConverterConfigure) Read(reader io.Reader) *Configure {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterConfigure) Lower(value *Configure) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Configure")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterConfigure) Write(writer io.Writer, value *Configure) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerConfigure struct{}

func (_ FfiDestroyerConfigure) Destroy(value *Configure) {
	value.Destroy()
}

type ContainerIdLike interface {
	AsContainerId(ty ContainerType) ContainerId
}
type ContainerIdLikeImpl struct {
	ffiObject FfiObject
}

func (_self *ContainerIdLikeImpl) AsContainerId(ty ContainerType) ContainerId {
	_pointer := _self.ffiObject.incrementPointer("ContainerIdLike")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterContainerIdINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_containeridlike_as_container_id(
				_pointer, FfiConverterContainerTypeINSTANCE.Lower(ty), _uniffiStatus),
		}
	}))
}
func (object *ContainerIdLikeImpl) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterContainerIdLike struct {
	handleMap *concurrentHandleMap[ContainerIdLike]
}

var FfiConverterContainerIdLikeINSTANCE = FfiConverterContainerIdLike{
	handleMap: newConcurrentHandleMap[ContainerIdLike](),
}

func (c FfiConverterContainerIdLike) Lift(pointer unsafe.Pointer) ContainerIdLike {
	result := &ContainerIdLikeImpl{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_loro_fn_clone_containeridlike(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_loro_fn_free_containeridlike(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*ContainerIdLikeImpl).Destroy)
	return result
}

func (c FfiConverterContainerIdLike) Read(reader io.Reader) ContainerIdLike {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterContainerIdLike) Lower(value ContainerIdLike) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := unsafe.Pointer(uintptr(c.handleMap.insert(value)))
	return pointer

}

func (c FfiConverterContainerIdLike) Write(writer io.Writer, value ContainerIdLike) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerContainerIdLike struct{}

func (_ FfiDestroyerContainerIdLike) Destroy(value ContainerIdLike) {
	if val, ok := value.(*ContainerIdLikeImpl); ok {
		val.Destroy()
	} else {
		panic("Expected *ContainerIdLikeImpl")
	}
}

//export loro_cgo_dispatchCallbackInterfaceContainerIdLikeMethod0
func loro_cgo_dispatchCallbackInterfaceContainerIdLikeMethod0(uniffiHandle C.uint64_t, ty C.RustBuffer, uniffiOutReturn *C.RustBuffer, callStatus *C.RustCallStatus) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterContainerIdLikeINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	res :=
		uniffiObj.AsContainerId(
			FfiConverterContainerTypeINSTANCE.Lift(GoRustBuffer{
				inner: ty,
			}),
		)

	*uniffiOutReturn = FfiConverterContainerIdINSTANCE.Lower(res)
}

var UniffiVTableCallbackInterfaceContainerIdLikeINSTANCE = C.UniffiVTableCallbackInterfaceContainerIdLike{
	asContainerId: (C.UniffiCallbackInterfaceContainerIdLikeMethod0)(C.loro_cgo_dispatchCallbackInterfaceContainerIdLikeMethod0),

	uniffiFree: (C.UniffiCallbackInterfaceFree)(C.loro_cgo_dispatchCallbackInterfaceContainerIdLikeFree),
}

//export loro_cgo_dispatchCallbackInterfaceContainerIdLikeFree
func loro_cgo_dispatchCallbackInterfaceContainerIdLikeFree(handle C.uint64_t) {
	FfiConverterContainerIdLikeINSTANCE.handleMap.remove(uint64(handle))
}

func (c FfiConverterContainerIdLike) register() {
	C.uniffi_loro_fn_init_callback_vtable_containeridlike(&UniffiVTableCallbackInterfaceContainerIdLikeINSTANCE)
}

type CursorInterface interface {
}
type Cursor struct {
	ffiObject FfiObject
}

func NewCursor(id *Id, container ContainerId, side Side, originPos uint32) *Cursor {
	return FfiConverterCursorINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_constructor_cursor_new(FfiConverterOptionalIdINSTANCE.Lower(id), FfiConverterContainerIdINSTANCE.Lower(container), FfiConverterSideINSTANCE.Lower(side), FfiConverterUint32INSTANCE.Lower(originPos), _uniffiStatus)
	}))
}

func (object *Cursor) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterCursor struct{}

var FfiConverterCursorINSTANCE = FfiConverterCursor{}

func (c FfiConverterCursor) Lift(pointer unsafe.Pointer) *Cursor {
	result := &Cursor{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_loro_fn_clone_cursor(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_loro_fn_free_cursor(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Cursor).Destroy)
	return result
}

func (c FfiConverterCursor) Read(reader io.Reader) *Cursor {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterCursor) Lower(value *Cursor) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Cursor")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterCursor) Write(writer io.Writer, value *Cursor) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerCursor struct{}

func (_ FfiDestroyerCursor) Destroy(value *Cursor) {
	value.Destroy()
}

type DiffBatchInterface interface {
	// Returns an iterator over the diffs in this batch, in the order they were added.
	//
	// The iterator yields tuples of `(&ContainerID, &Diff)` where:
	// - `ContainerID` is the ID of the container that was modified
	// - `Diff` contains the actual changes made to that container
	//
	// The order of the diffs is preserved from when they were originally added to the batch.
	GetDiff() []ContainerIdAndDiff
	// Push a new event to the batch.
	//
	// If the cid already exists in the batch, return Err
	Push(cid ContainerId, diff Diff) *Diff
}
type DiffBatch struct {
	ffiObject FfiObject
}

func NewDiffBatch() *DiffBatch {
	return FfiConverterDiffBatchINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_constructor_diffbatch_new(_uniffiStatus)
	}))
}

// Returns an iterator over the diffs in this batch, in the order they were added.
//
// The iterator yields tuples of `(&ContainerID, &Diff)` where:
// - `ContainerID` is the ID of the container that was modified
// - `Diff` contains the actual changes made to that container
//
// The order of the diffs is preserved from when they were originally added to the batch.
func (_self *DiffBatch) GetDiff() []ContainerIdAndDiff {
	_pointer := _self.ffiObject.incrementPointer("*DiffBatch")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceContainerIdAndDiffINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_diffbatch_get_diff(
				_pointer, _uniffiStatus),
		}
	}))
}

// Push a new event to the batch.
//
// If the cid already exists in the batch, return Err
func (_self *DiffBatch) Push(cid ContainerId, diff Diff) *Diff {
	_pointer := _self.ffiObject.incrementPointer("*DiffBatch")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalDiffINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_diffbatch_push(
				_pointer, FfiConverterContainerIdINSTANCE.Lower(cid), FfiConverterDiffINSTANCE.Lower(diff), _uniffiStatus),
		}
	}))
}
func (object *DiffBatch) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterDiffBatch struct{}

var FfiConverterDiffBatchINSTANCE = FfiConverterDiffBatch{}

func (c FfiConverterDiffBatch) Lift(pointer unsafe.Pointer) *DiffBatch {
	result := &DiffBatch{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_loro_fn_clone_diffbatch(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_loro_fn_free_diffbatch(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*DiffBatch).Destroy)
	return result
}

func (c FfiConverterDiffBatch) Read(reader io.Reader) *DiffBatch {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterDiffBatch) Lower(value *DiffBatch) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*DiffBatch")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterDiffBatch) Write(writer io.Writer, value *DiffBatch) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerDiffBatch struct{}

func (_ FfiDestroyerDiffBatch) Destroy(value *DiffBatch) {
	value.Destroy()
}

type EphemeralStoreInterface interface {
	Apply(data []byte)
	Delete(key string)
	Encode(key string) []byte
	EncodeAll() []byte
	Get(key string) *LoroValue
	GetAllStates() map[string]LoroValue
	Keys() []string
	RemoveOutdated()
	Set(key string, value LoroValueLike)
	Subscribe(listener EphemeralSubscriber) *Subscription
	SubscribeLocalUpdate(listener LocalEphemeralListener) *Subscription
}
type EphemeralStore struct {
	ffiObject FfiObject
}

func NewEphemeralStore(timeout int64) *EphemeralStore {
	return FfiConverterEphemeralStoreINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_constructor_ephemeralstore_new(FfiConverterInt64INSTANCE.Lower(timeout), _uniffiStatus)
	}))
}

func (_self *EphemeralStore) Apply(data []byte) {
	_pointer := _self.ffiObject.incrementPointer("*EphemeralStore")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_ephemeralstore_apply(
			_pointer, FfiConverterBytesINSTANCE.Lower(data), _uniffiStatus)
		return false
	})
}

func (_self *EphemeralStore) Delete(key string) {
	_pointer := _self.ffiObject.incrementPointer("*EphemeralStore")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_ephemeralstore_delete(
			_pointer, FfiConverterStringINSTANCE.Lower(key), _uniffiStatus)
		return false
	})
}

func (_self *EphemeralStore) Encode(key string) []byte {
	_pointer := _self.ffiObject.incrementPointer("*EphemeralStore")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBytesINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_ephemeralstore_encode(
				_pointer, FfiConverterStringINSTANCE.Lower(key), _uniffiStatus),
		}
	}))
}

func (_self *EphemeralStore) EncodeAll() []byte {
	_pointer := _self.ffiObject.incrementPointer("*EphemeralStore")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBytesINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_ephemeralstore_encode_all(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *EphemeralStore) Get(key string) *LoroValue {
	_pointer := _self.ffiObject.incrementPointer("*EphemeralStore")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalLoroValueINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_ephemeralstore_get(
				_pointer, FfiConverterStringINSTANCE.Lower(key), _uniffiStatus),
		}
	}))
}

func (_self *EphemeralStore) GetAllStates() map[string]LoroValue {
	_pointer := _self.ffiObject.incrementPointer("*EphemeralStore")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterMapStringLoroValueINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_ephemeralstore_get_all_states(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *EphemeralStore) Keys() []string {
	_pointer := _self.ffiObject.incrementPointer("*EphemeralStore")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_ephemeralstore_keys(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *EphemeralStore) RemoveOutdated() {
	_pointer := _self.ffiObject.incrementPointer("*EphemeralStore")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_ephemeralstore_remove_outdated(
			_pointer, _uniffiStatus)
		return false
	})
}

func (_self *EphemeralStore) Set(key string, value LoroValueLike) {
	_pointer := _self.ffiObject.incrementPointer("*EphemeralStore")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_ephemeralstore_set(
			_pointer, FfiConverterStringINSTANCE.Lower(key), FfiConverterLoroValueLikeINSTANCE.Lower(value), _uniffiStatus)
		return false
	})
}

func (_self *EphemeralStore) Subscribe(listener EphemeralSubscriber) *Subscription {
	_pointer := _self.ffiObject.incrementPointer("*EphemeralStore")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSubscriptionINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_ephemeralstore_subscribe(
			_pointer, FfiConverterEphemeralSubscriberINSTANCE.Lower(listener), _uniffiStatus)
	}))
}

func (_self *EphemeralStore) SubscribeLocalUpdate(listener LocalEphemeralListener) *Subscription {
	_pointer := _self.ffiObject.incrementPointer("*EphemeralStore")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSubscriptionINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_ephemeralstore_subscribe_local_update(
			_pointer, FfiConverterLocalEphemeralListenerINSTANCE.Lower(listener), _uniffiStatus)
	}))
}
func (object *EphemeralStore) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterEphemeralStore struct{}

var FfiConverterEphemeralStoreINSTANCE = FfiConverterEphemeralStore{}

func (c FfiConverterEphemeralStore) Lift(pointer unsafe.Pointer) *EphemeralStore {
	result := &EphemeralStore{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_loro_fn_clone_ephemeralstore(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_loro_fn_free_ephemeralstore(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*EphemeralStore).Destroy)
	return result
}

func (c FfiConverterEphemeralStore) Read(reader io.Reader) *EphemeralStore {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterEphemeralStore) Lower(value *EphemeralStore) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*EphemeralStore")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterEphemeralStore) Write(writer io.Writer, value *EphemeralStore) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerEphemeralStore struct{}

func (_ FfiDestroyerEphemeralStore) Destroy(value *EphemeralStore) {
	value.Destroy()
}

type EphemeralSubscriber interface {
	OnEphemeralEvent(event EphemeralStoreEvent)
}
type EphemeralSubscriberImpl struct {
	ffiObject FfiObject
}

func (_self *EphemeralSubscriberImpl) OnEphemeralEvent(event EphemeralStoreEvent) {
	_pointer := _self.ffiObject.incrementPointer("EphemeralSubscriber")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_ephemeralsubscriber_on_ephemeral_event(
			_pointer, FfiConverterEphemeralStoreEventINSTANCE.Lower(event), _uniffiStatus)
		return false
	})
}
func (object *EphemeralSubscriberImpl) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterEphemeralSubscriber struct {
	handleMap *concurrentHandleMap[EphemeralSubscriber]
}

var FfiConverterEphemeralSubscriberINSTANCE = FfiConverterEphemeralSubscriber{
	handleMap: newConcurrentHandleMap[EphemeralSubscriber](),
}

func (c FfiConverterEphemeralSubscriber) Lift(pointer unsafe.Pointer) EphemeralSubscriber {
	result := &EphemeralSubscriberImpl{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_loro_fn_clone_ephemeralsubscriber(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_loro_fn_free_ephemeralsubscriber(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*EphemeralSubscriberImpl).Destroy)
	return result
}

func (c FfiConverterEphemeralSubscriber) Read(reader io.Reader) EphemeralSubscriber {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterEphemeralSubscriber) Lower(value EphemeralSubscriber) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := unsafe.Pointer(uintptr(c.handleMap.insert(value)))
	return pointer

}

func (c FfiConverterEphemeralSubscriber) Write(writer io.Writer, value EphemeralSubscriber) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerEphemeralSubscriber struct{}

func (_ FfiDestroyerEphemeralSubscriber) Destroy(value EphemeralSubscriber) {
	if val, ok := value.(*EphemeralSubscriberImpl); ok {
		val.Destroy()
	} else {
		panic("Expected *EphemeralSubscriberImpl")
	}
}

//export loro_cgo_dispatchCallbackInterfaceEphemeralSubscriberMethod0
func loro_cgo_dispatchCallbackInterfaceEphemeralSubscriberMethod0(uniffiHandle C.uint64_t, event C.RustBuffer, uniffiOutReturn *C.void, callStatus *C.RustCallStatus) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterEphemeralSubscriberINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	uniffiObj.OnEphemeralEvent(
		FfiConverterEphemeralStoreEventINSTANCE.Lift(GoRustBuffer{
			inner: event,
		}),
	)

}

var UniffiVTableCallbackInterfaceEphemeralSubscriberINSTANCE = C.UniffiVTableCallbackInterfaceEphemeralSubscriber{
	onEphemeralEvent: (C.UniffiCallbackInterfaceEphemeralSubscriberMethod0)(C.loro_cgo_dispatchCallbackInterfaceEphemeralSubscriberMethod0),

	uniffiFree: (C.UniffiCallbackInterfaceFree)(C.loro_cgo_dispatchCallbackInterfaceEphemeralSubscriberFree),
}

//export loro_cgo_dispatchCallbackInterfaceEphemeralSubscriberFree
func loro_cgo_dispatchCallbackInterfaceEphemeralSubscriberFree(handle C.uint64_t) {
	FfiConverterEphemeralSubscriberINSTANCE.handleMap.remove(uint64(handle))
}

func (c FfiConverterEphemeralSubscriber) register() {
	C.uniffi_loro_fn_init_callback_vtable_ephemeralsubscriber(&UniffiVTableCallbackInterfaceEphemeralSubscriberINSTANCE)
}

type FirstCommitFromPeerCallback interface {
	OnFirstCommitFromPeer(payload FirstCommitFromPeerPayload)
}
type FirstCommitFromPeerCallbackImpl struct {
	ffiObject FfiObject
}

func (_self *FirstCommitFromPeerCallbackImpl) OnFirstCommitFromPeer(payload FirstCommitFromPeerPayload) {
	_pointer := _self.ffiObject.incrementPointer("FirstCommitFromPeerCallback")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_firstcommitfrompeercallback_on_first_commit_from_peer(
			_pointer, FfiConverterFirstCommitFromPeerPayloadINSTANCE.Lower(payload), _uniffiStatus)
		return false
	})
}
func (object *FirstCommitFromPeerCallbackImpl) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterFirstCommitFromPeerCallback struct {
	handleMap *concurrentHandleMap[FirstCommitFromPeerCallback]
}

var FfiConverterFirstCommitFromPeerCallbackINSTANCE = FfiConverterFirstCommitFromPeerCallback{
	handleMap: newConcurrentHandleMap[FirstCommitFromPeerCallback](),
}

func (c FfiConverterFirstCommitFromPeerCallback) Lift(pointer unsafe.Pointer) FirstCommitFromPeerCallback {
	result := &FirstCommitFromPeerCallbackImpl{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_loro_fn_clone_firstcommitfrompeercallback(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_loro_fn_free_firstcommitfrompeercallback(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*FirstCommitFromPeerCallbackImpl).Destroy)
	return result
}

func (c FfiConverterFirstCommitFromPeerCallback) Read(reader io.Reader) FirstCommitFromPeerCallback {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterFirstCommitFromPeerCallback) Lower(value FirstCommitFromPeerCallback) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := unsafe.Pointer(uintptr(c.handleMap.insert(value)))
	return pointer

}

func (c FfiConverterFirstCommitFromPeerCallback) Write(writer io.Writer, value FirstCommitFromPeerCallback) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerFirstCommitFromPeerCallback struct{}

func (_ FfiDestroyerFirstCommitFromPeerCallback) Destroy(value FirstCommitFromPeerCallback) {
	if val, ok := value.(*FirstCommitFromPeerCallbackImpl); ok {
		val.Destroy()
	} else {
		panic("Expected *FirstCommitFromPeerCallbackImpl")
	}
}

//export loro_cgo_dispatchCallbackInterfaceFirstCommitFromPeerCallbackMethod0
func loro_cgo_dispatchCallbackInterfaceFirstCommitFromPeerCallbackMethod0(uniffiHandle C.uint64_t, payload C.RustBuffer, uniffiOutReturn *C.void, callStatus *C.RustCallStatus) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterFirstCommitFromPeerCallbackINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	uniffiObj.OnFirstCommitFromPeer(
		FfiConverterFirstCommitFromPeerPayloadINSTANCE.Lift(GoRustBuffer{
			inner: payload,
		}),
	)

}

var UniffiVTableCallbackInterfaceFirstCommitFromPeerCallbackINSTANCE = C.UniffiVTableCallbackInterfaceFirstCommitFromPeerCallback{
	onFirstCommitFromPeer: (C.UniffiCallbackInterfaceFirstCommitFromPeerCallbackMethod0)(C.loro_cgo_dispatchCallbackInterfaceFirstCommitFromPeerCallbackMethod0),

	uniffiFree: (C.UniffiCallbackInterfaceFree)(C.loro_cgo_dispatchCallbackInterfaceFirstCommitFromPeerCallbackFree),
}

//export loro_cgo_dispatchCallbackInterfaceFirstCommitFromPeerCallbackFree
func loro_cgo_dispatchCallbackInterfaceFirstCommitFromPeerCallbackFree(handle C.uint64_t) {
	FfiConverterFirstCommitFromPeerCallbackINSTANCE.handleMap.remove(uint64(handle))
}

func (c FfiConverterFirstCommitFromPeerCallback) register() {
	C.uniffi_loro_fn_init_callback_vtable_firstcommitfrompeercallback(&UniffiVTableCallbackInterfaceFirstCommitFromPeerCallbackINSTANCE)
}

type FractionalIndexInterface interface {
	ToString() string
}
type FractionalIndex struct {
	ffiObject FfiObject
}

func FractionalIndexFromBytes(bytes []byte) *FractionalIndex {
	return FfiConverterFractionalIndexINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_constructor_fractionalindex_from_bytes(FfiConverterBytesINSTANCE.Lower(bytes), _uniffiStatus)
	}))
}

func FractionalIndexFromHexString(str string) *FractionalIndex {
	return FfiConverterFractionalIndexINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_constructor_fractionalindex_from_hex_string(FfiConverterStringINSTANCE.Lower(str), _uniffiStatus)
	}))
}

func (_self *FractionalIndex) ToString() string {
	_pointer := _self.ffiObject.incrementPointer("*FractionalIndex")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_fractionalindex_to_string(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *FractionalIndex) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterFractionalIndex struct{}

var FfiConverterFractionalIndexINSTANCE = FfiConverterFractionalIndex{}

func (c FfiConverterFractionalIndex) Lift(pointer unsafe.Pointer) *FractionalIndex {
	result := &FractionalIndex{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_loro_fn_clone_fractionalindex(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_loro_fn_free_fractionalindex(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*FractionalIndex).Destroy)
	return result
}

func (c FfiConverterFractionalIndex) Read(reader io.Reader) *FractionalIndex {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterFractionalIndex) Lower(value *FractionalIndex) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*FractionalIndex")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterFractionalIndex) Write(writer io.Writer, value *FractionalIndex) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerFractionalIndex struct{}

func (_ FfiDestroyerFractionalIndex) Destroy(value *FractionalIndex) {
	value.Destroy()
}

type FrontiersInterface interface {
	Encode() []byte
	Eq(other *Frontiers) bool
}
type Frontiers struct {
	ffiObject FfiObject
}

func NewFrontiers() *Frontiers {
	return FfiConverterFrontiersINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_constructor_frontiers_new(_uniffiStatus)
	}))
}

func FrontiersDecode(bytes []byte) (*Frontiers, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_constructor_frontiers_decode(FfiConverterBytesINSTANCE.Lower(bytes), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Frontiers
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterFrontiersINSTANCE.Lift(_uniffiRV), nil
	}
}

func FrontiersFromId(id Id) *Frontiers {
	return FfiConverterFrontiersINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_constructor_frontiers_from_id(FfiConverterIdINSTANCE.Lower(id), _uniffiStatus)
	}))
}

func FrontiersFromIds(ids []Id) *Frontiers {
	return FfiConverterFrontiersINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_constructor_frontiers_from_ids(FfiConverterSequenceIdINSTANCE.Lower(ids), _uniffiStatus)
	}))
}

func (_self *Frontiers) Encode() []byte {
	_pointer := _self.ffiObject.incrementPointer("*Frontiers")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBytesINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_frontiers_encode(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Frontiers) Eq(other *Frontiers) bool {
	_pointer := _self.ffiObject.incrementPointer("*Frontiers")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_loro_fn_method_frontiers_eq(
			_pointer, FfiConverterFrontiersINSTANCE.Lower(other), _uniffiStatus)
	}))
}
func (object *Frontiers) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterFrontiers struct{}

var FfiConverterFrontiersINSTANCE = FfiConverterFrontiers{}

func (c FfiConverterFrontiers) Lift(pointer unsafe.Pointer) *Frontiers {
	result := &Frontiers{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_loro_fn_clone_frontiers(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_loro_fn_free_frontiers(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Frontiers).Destroy)
	return result
}

func (c FfiConverterFrontiers) Read(reader io.Reader) *Frontiers {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterFrontiers) Lower(value *Frontiers) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Frontiers")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterFrontiers) Write(writer io.Writer, value *Frontiers) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerFrontiers struct{}

func (_ FfiDestroyerFrontiers) Destroy(value *Frontiers) {
	value.Destroy()
}

type LocalEphemeralListener interface {
	OnEphemeralUpdate(update []byte)
}
type LocalEphemeralListenerImpl struct {
	ffiObject FfiObject
}

func (_self *LocalEphemeralListenerImpl) OnEphemeralUpdate(update []byte) {
	_pointer := _self.ffiObject.incrementPointer("LocalEphemeralListener")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_localephemerallistener_on_ephemeral_update(
			_pointer, FfiConverterBytesINSTANCE.Lower(update), _uniffiStatus)
		return false
	})
}
func (object *LocalEphemeralListenerImpl) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterLocalEphemeralListener struct {
	handleMap *concurrentHandleMap[LocalEphemeralListener]
}

var FfiConverterLocalEphemeralListenerINSTANCE = FfiConverterLocalEphemeralListener{
	handleMap: newConcurrentHandleMap[LocalEphemeralListener](),
}

func (c FfiConverterLocalEphemeralListener) Lift(pointer unsafe.Pointer) LocalEphemeralListener {
	result := &LocalEphemeralListenerImpl{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_loro_fn_clone_localephemerallistener(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_loro_fn_free_localephemerallistener(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*LocalEphemeralListenerImpl).Destroy)
	return result
}

func (c FfiConverterLocalEphemeralListener) Read(reader io.Reader) LocalEphemeralListener {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterLocalEphemeralListener) Lower(value LocalEphemeralListener) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := unsafe.Pointer(uintptr(c.handleMap.insert(value)))
	return pointer

}

func (c FfiConverterLocalEphemeralListener) Write(writer io.Writer, value LocalEphemeralListener) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerLocalEphemeralListener struct{}

func (_ FfiDestroyerLocalEphemeralListener) Destroy(value LocalEphemeralListener) {
	if val, ok := value.(*LocalEphemeralListenerImpl); ok {
		val.Destroy()
	} else {
		panic("Expected *LocalEphemeralListenerImpl")
	}
}

//export loro_cgo_dispatchCallbackInterfaceLocalEphemeralListenerMethod0
func loro_cgo_dispatchCallbackInterfaceLocalEphemeralListenerMethod0(uniffiHandle C.uint64_t, update C.RustBuffer, uniffiOutReturn *C.void, callStatus *C.RustCallStatus) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterLocalEphemeralListenerINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	uniffiObj.OnEphemeralUpdate(
		FfiConverterBytesINSTANCE.Lift(GoRustBuffer{
			inner: update,
		}),
	)

}

var UniffiVTableCallbackInterfaceLocalEphemeralListenerINSTANCE = C.UniffiVTableCallbackInterfaceLocalEphemeralListener{
	onEphemeralUpdate: (C.UniffiCallbackInterfaceLocalEphemeralListenerMethod0)(C.loro_cgo_dispatchCallbackInterfaceLocalEphemeralListenerMethod0),

	uniffiFree: (C.UniffiCallbackInterfaceFree)(C.loro_cgo_dispatchCallbackInterfaceLocalEphemeralListenerFree),
}

//export loro_cgo_dispatchCallbackInterfaceLocalEphemeralListenerFree
func loro_cgo_dispatchCallbackInterfaceLocalEphemeralListenerFree(handle C.uint64_t) {
	FfiConverterLocalEphemeralListenerINSTANCE.handleMap.remove(uint64(handle))
}

func (c FfiConverterLocalEphemeralListener) register() {
	C.uniffi_loro_fn_init_callback_vtable_localephemerallistener(&UniffiVTableCallbackInterfaceLocalEphemeralListenerINSTANCE)
}

type LocalUpdateCallback interface {
	OnLocalUpdate(update []byte)
}
type LocalUpdateCallbackImpl struct {
	ffiObject FfiObject
}

func (_self *LocalUpdateCallbackImpl) OnLocalUpdate(update []byte) {
	_pointer := _self.ffiObject.incrementPointer("LocalUpdateCallback")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_localupdatecallback_on_local_update(
			_pointer, FfiConverterBytesINSTANCE.Lower(update), _uniffiStatus)
		return false
	})
}
func (object *LocalUpdateCallbackImpl) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterLocalUpdateCallback struct {
	handleMap *concurrentHandleMap[LocalUpdateCallback]
}

var FfiConverterLocalUpdateCallbackINSTANCE = FfiConverterLocalUpdateCallback{
	handleMap: newConcurrentHandleMap[LocalUpdateCallback](),
}

func (c FfiConverterLocalUpdateCallback) Lift(pointer unsafe.Pointer) LocalUpdateCallback {
	result := &LocalUpdateCallbackImpl{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_loro_fn_clone_localupdatecallback(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_loro_fn_free_localupdatecallback(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*LocalUpdateCallbackImpl).Destroy)
	return result
}

func (c FfiConverterLocalUpdateCallback) Read(reader io.Reader) LocalUpdateCallback {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterLocalUpdateCallback) Lower(value LocalUpdateCallback) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := unsafe.Pointer(uintptr(c.handleMap.insert(value)))
	return pointer

}

func (c FfiConverterLocalUpdateCallback) Write(writer io.Writer, value LocalUpdateCallback) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerLocalUpdateCallback struct{}

func (_ FfiDestroyerLocalUpdateCallback) Destroy(value LocalUpdateCallback) {
	if val, ok := value.(*LocalUpdateCallbackImpl); ok {
		val.Destroy()
	} else {
		panic("Expected *LocalUpdateCallbackImpl")
	}
}

//export loro_cgo_dispatchCallbackInterfaceLocalUpdateCallbackMethod0
func loro_cgo_dispatchCallbackInterfaceLocalUpdateCallbackMethod0(uniffiHandle C.uint64_t, update C.RustBuffer, uniffiOutReturn *C.void, callStatus *C.RustCallStatus) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterLocalUpdateCallbackINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	uniffiObj.OnLocalUpdate(
		FfiConverterBytesINSTANCE.Lift(GoRustBuffer{
			inner: update,
		}),
	)

}

var UniffiVTableCallbackInterfaceLocalUpdateCallbackINSTANCE = C.UniffiVTableCallbackInterfaceLocalUpdateCallback{
	onLocalUpdate: (C.UniffiCallbackInterfaceLocalUpdateCallbackMethod0)(C.loro_cgo_dispatchCallbackInterfaceLocalUpdateCallbackMethod0),

	uniffiFree: (C.UniffiCallbackInterfaceFree)(C.loro_cgo_dispatchCallbackInterfaceLocalUpdateCallbackFree),
}

//export loro_cgo_dispatchCallbackInterfaceLocalUpdateCallbackFree
func loro_cgo_dispatchCallbackInterfaceLocalUpdateCallbackFree(handle C.uint64_t) {
	FfiConverterLocalUpdateCallbackINSTANCE.handleMap.remove(uint64(handle))
}

func (c FfiConverterLocalUpdateCallback) register() {
	C.uniffi_loro_fn_init_callback_vtable_localupdatecallback(&UniffiVTableCallbackInterfaceLocalUpdateCallbackINSTANCE)
}

type LoroCounterInterface interface {
	// Decrement the counter by the given value.
	Decrement(value float64) error
	// Get the LoroDoc from this container
	Doc() **LoroDoc
	// If a detached container is attached, this method will return its corresponding attached handler.
	GetAttached() **LoroCounter
	// Get the current value of the counter.
	GetValue() float64
	// Return container id of the Counter.
	Id() ContainerId
	// Increment the counter by the given value.
	Increment(value float64) error
	// Whether the container is attached to a document
	//
	// The edits on a detached container will not be persisted.
	// To attach the container to the document, please insert it into an attached container.
	IsAttached() bool
	// Whether the container is deleted.
	IsDeleted() bool
}
type LoroCounter struct {
	ffiObject FfiObject
}

// Create a new Counter.
func NewLoroCounter() *LoroCounter {
	return FfiConverterLoroCounterINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_constructor_lorocounter_new(_uniffiStatus)
	}))
}

// Decrement the counter by the given value.
func (_self *LoroCounter) Decrement(value float64) error {
	_pointer := _self.ffiObject.incrementPointer("*LoroCounter")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorocounter_decrement(
			_pointer, FfiConverterFloat64INSTANCE.Lower(value), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Get the LoroDoc from this container
func (_self *LoroCounter) Doc() **LoroDoc {
	_pointer := _self.ffiObject.incrementPointer("*LoroCounter")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalLoroDocINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorocounter_doc(
				_pointer, _uniffiStatus),
		}
	}))
}

// If a detached container is attached, this method will return its corresponding attached handler.
func (_self *LoroCounter) GetAttached() **LoroCounter {
	_pointer := _self.ffiObject.incrementPointer("*LoroCounter")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalLoroCounterINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorocounter_get_attached(
				_pointer, _uniffiStatus),
		}
	}))
}

// Get the current value of the counter.
func (_self *LoroCounter) GetValue() float64 {
	_pointer := _self.ffiObject.incrementPointer("*LoroCounter")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterFloat64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.double {
		return C.uniffi_loro_fn_method_lorocounter_get_value(
			_pointer, _uniffiStatus)
	}))
}

// Return container id of the Counter.
func (_self *LoroCounter) Id() ContainerId {
	_pointer := _self.ffiObject.incrementPointer("*LoroCounter")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterContainerIdINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorocounter_id(
				_pointer, _uniffiStatus),
		}
	}))
}

// Increment the counter by the given value.
func (_self *LoroCounter) Increment(value float64) error {
	_pointer := _self.ffiObject.incrementPointer("*LoroCounter")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorocounter_increment(
			_pointer, FfiConverterFloat64INSTANCE.Lower(value), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Whether the container is attached to a document
//
// The edits on a detached container will not be persisted.
// To attach the container to the document, please insert it into an attached container.
func (_self *LoroCounter) IsAttached() bool {
	_pointer := _self.ffiObject.incrementPointer("*LoroCounter")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_loro_fn_method_lorocounter_is_attached(
			_pointer, _uniffiStatus)
	}))
}

// Whether the container is deleted.
func (_self *LoroCounter) IsDeleted() bool {
	_pointer := _self.ffiObject.incrementPointer("*LoroCounter")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_loro_fn_method_lorocounter_is_deleted(
			_pointer, _uniffiStatus)
	}))
}
func (object *LoroCounter) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterLoroCounter struct{}

var FfiConverterLoroCounterINSTANCE = FfiConverterLoroCounter{}

func (c FfiConverterLoroCounter) Lift(pointer unsafe.Pointer) *LoroCounter {
	result := &LoroCounter{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_loro_fn_clone_lorocounter(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_loro_fn_free_lorocounter(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*LoroCounter).Destroy)
	return result
}

func (c FfiConverterLoroCounter) Read(reader io.Reader) *LoroCounter {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterLoroCounter) Lower(value *LoroCounter) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*LoroCounter")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterLoroCounter) Write(writer io.Writer, value *LoroCounter) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerLoroCounter struct{}

func (_ FfiDestroyerLoroCounter) Destroy(value *LoroCounter) {
	value.Destroy()
}

// `LoroDoc` is the entry for the whole document.
// When it's dropped, all the associated [`Handler`]s will be invalidated.
//
// **Important:** Loro is a pure library and does not handle network protocols.
// It is the responsibility of the user to manage the storage, loading, and synchronization
// of the bytes exported by Loro in a manner suitable for their specific environment.
type LoroDocInterface interface {
	// Apply a diff to the current document state.
	//
	// Internally, it will apply the diff to the current state.
	ApplyDiff(diff *DiffBatch) error
	// Attach the document state to the latest known version.
	//
	// > The document becomes detached during a `checkout` operation.
	// > Being `detached` implies that the `DocState` is not synchronized with the latest version of the `OpLog`.
	// > In a detached state, the document is not editable, and any `import` operations will be
	// > recorded in the `OpLog` without being applied to the `DocState`.
	Attach()
	// Check the correctness of the document state by comparing it with the state
	// calculated by applying all the history.
	CheckStateCorrectnessSlow()
	// Checkout the `DocState` to a specific version.
	//
	// The document becomes detached during a `checkout` operation.
	// Being `detached` implies that the `DocState` is not synchronized with the latest version of the `OpLog`.
	// In a detached state, the document is not editable, and any `import` operations will be
	// recorded in the `OpLog` without being applied to the `DocState`.
	//
	// You should call `attach` to attach the `DocState` to the latest version of `OpLog`.
	Checkout(frontiers *Frontiers) error
	// Checkout the `DocState` to the latest version.
	//
	// > The document becomes detached during a `checkout` operation.
	// > Being `detached` implies that the `DocState` is not synchronized with the latest version of the `OpLog`.
	// > In a detached state, the document is not editable, and any `import` operations will be
	// > recorded in the `OpLog` without being applied to the `DocState`.
	//
	// This has the same effect as `attach`.
	CheckoutToLatest()
	// Clear the options of the next commit.
	ClearNextCommitOptions()
	// Compare the frontiers with the current OpLog's version.
	//
	// If `other` contains any version that's not contained in the current OpLog, return [Ordering::Less].
	CmpWithFrontiers(other *Frontiers) Ordering
	// Commit the cumulative auto commit transaction.
	//
	// There is a transaction behind every operation.
	// The events will be emitted after a transaction is committed. A transaction is committed when:
	//
	// - `doc.commit()` is called.
	// - `doc.export(mode)` is called.
	// - `doc.import(data)` is called.
	// - `doc.checkout(version)` is called.
	Commit()
	CommitWith(options CommitOptions)
	// Encoded all ops and history cache to bytes and store them in the kv store.
	//
	// The parsed ops will be dropped
	CompactChangeStore()
	// Get the configurations of the document.
	Config() *Configure
	// Configures the default text style for the document.
	//
	// This method sets the default text style configuration for the document when using LoroText.
	// If `None` is provided, the default style is reset.
	//
	// # Parameters
	//
	// - `text_style`: The style configuration to set as the default. `None` to reset.
	ConfigDefaultTextStyle(textStyle *StyleConfig)
	// Set the rich text format configuration of the document.
	//
	// You need to config it if you use rich text `mark` method.
	// Specifically, you need to config the `expand` property of each style.
	//
	// Expand is used to specify the behavior of expanding when new text is inserted at the
	// beginning or end of the style.
	ConfigTextStyle(textStyle *StyleConfigMap)
	// Delete all content from a root container and hide it from the document.
	//
	// When a root container is empty and hidden:
	// - It won't show up in `get_deep_value()` results
	// - It won't be included in document snapshots
	//
	// Only works on root containers (containers without parents).
	DeleteRootContainer(cid ContainerId)
	// Force the document enter the detached mode.
	//
	// In this mode, when you importing new updates, the [loro_internal::DocState] will not be changed.
	//
	// Learn more at https://loro.dev/docs/advanced/doc_state_and_oplog#attacheddetached-status
	Detach()
	// Calculate the diff between two versions
	Diff(a *Frontiers, b *Frontiers) (*DiffBatch, error)
	// Exports changes within the specified ID span to JSON schema format.
	//
	// The JSON schema format produced by this method is identical to the one generated by `export_json_updates`.
	// It ensures deterministic output, making it ideal for hash calculations and integrity checks.
	//
	// This method can also export pending changes from the uncommitted transaction that have not yet been applied to the OpLog.
	//
	// This method will NOT trigger a new commit implicitly.
	ExportJsonInIdSpan(idSpan IdSpan) []string
	// Export the current state with json-string format of the document.
	ExportJsonUpdates(startVv *VersionVector, endVv *VersionVector) string
	// Export the current state with json-string format of the document, without peer compression.
	//
	// Compared to [`export_json_updates`], this method does not compress the peer IDs in the updates.
	// So the operations are easier to be processed by application code.
	ExportJsonUpdatesWithoutPeerCompression(startVv *VersionVector, endVv *VersionVector) string
	ExportShallowSnapshot(frontiers *Frontiers) ([]byte, error)
	// Export the current state and history of the document.
	ExportSnapshot() ([]byte, error)
	ExportSnapshotAt(frontiers *Frontiers) ([]byte, error)
	ExportStateOnly(frontiers **Frontiers) ([]byte, error)
	// Export all the ops not included in the given `VersionVector`
	ExportUpdates(vv *VersionVector) ([]byte, error)
	ExportUpdatesInRange(spans []IdSpan) ([]byte, error)
	// Find the operation id spans that between the `from` version and the `to` version.
	FindIdSpansBetween(from *Frontiers, to *Frontiers) VersionVectorDiff
	// Duplicate the document with a different PeerID
	//
	// The time complexity and space complexity of this operation are both O(n),
	//
	// When called in detached mode, it will fork at the current state frontiers.
	// It will have the same effect as `fork_at(&self.state_frontiers())`.
	Fork() *LoroDoc
	// Fork the document at the given frontiers.
	//
	// The created doc will only contain the history before the specified frontiers.
	ForkAt(frontiers *Frontiers) *LoroDoc
	// Free the cached diff calculator that is used for checkout.
	FreeDiffCalculator()
	// Free the history cache that is used for making checkout faster.
	//
	// If you use checkout that switching to an old/concurrent version, the history cache will be built.
	// You can free it by calling this method.
	FreeHistoryCache()
	// Convert `Frontiers` into `VersionVector`
	FrontiersToVv(frontiers *Frontiers) **VersionVector
	// Get the handler by the path.
	GetByPath(path []Index) **ValueOrContainer
	// The path can be specified in different ways depending on the container type:
	//
	// For Tree:
	// 1. Using node IDs: `tree/{node_id}/property`
	// 2. Using indices: `tree/0/1/property`
	//
	// For List and MovableList:
	// - Using indices: `list/0` or `list/1/property`
	//
	// For Map:
	// - Using keys: `map/key` or `map/nested/property`
	//
	// For tree structures, index-based paths follow depth-first traversal order.
	// The indices start from 0 and represent the position of a node among its siblings.
	//
	// # Examples
	// ```
	// # use loro::{LoroDoc, LoroValue};
	// let doc = LoroDoc::new();
	//
	// // Tree example
	// let tree = doc.get_tree("tree");
	// let root = tree.create(None).unwrap();
	// tree.get_meta(root).unwrap().insert("name", "root").unwrap();
	// // Access tree by ID or index
	// let name1 = doc.get_by_str_path(&format!("tree/{}/name", root)).unwrap().into_value().unwrap();
	// let name2 = doc.get_by_str_path("tree/0/name").unwrap().into_value().unwrap();
	// assert_eq!(name1, name2);
	//
	// // List example
	// let list = doc.get_list("list");
	// list.insert(0, "first").unwrap();
	// list.insert(1, "second").unwrap();
	// // Access list by index
	// let item = doc.get_by_str_path("list/0");
	// assert_eq!(item.unwrap().into_value().unwrap().into_string().unwrap(), "first".into());
	//
	// // Map example
	// let map = doc.get_map("map");
	// map.insert("key", "value").unwrap();
	// // Access map by key
	// let value = doc.get_by_str_path("map/key");
	// assert_eq!(value.unwrap().into_value().unwrap().into_string().unwrap(), "value".into());
	//
	// // MovableList example
	// let mlist = doc.get_movable_list("mlist");
	// mlist.insert(0, "item").unwrap();
	// // Access movable list by index
	// let item = doc.get_by_str_path("mlist/0");
	// assert_eq!(item.unwrap().into_value().unwrap().into_string().unwrap(), "item".into());
	// ```
	GetByStrPath(path string) **ValueOrContainer
	// Get `Change` at the given id.
	//
	// `Change` is a grouped continuous operations that share the same id, timestamp, commit message.
	//
	// - The id of the `Change` is the id of its first op.
	// - The second op's id is `{ peer: change.id.peer, counter: change.id.counter + 1 }`
	//
	// The same applies on `Lamport`:
	//
	// - The lamport of the `Change` is the lamport of its first op.
	// - The second op's lamport is `change.lamport + 1`
	//
	// The length of the `Change` is how many operations it contains
	GetChange(id Id) *ChangeMeta
	// Gets container IDs modified in the given ID range.
	//
	// **NOTE:** This method will implicitly commit.
	//
	// This method can be used in conjunction with `doc.travel_change_ancestors()` to traverse
	// the history and identify all changes that affected specific containers.
	//
	// # Arguments
	//
	// * `id` - The starting ID of the change range
	// * `len` - The length of the change range to check
	GetChangedContainersIn(id Id, len uint32) []ContainerId
	// Get a [LoroCounter] by container id.
	//
	// If the provided id is string, it will be converted into a root container id with the name of the string.
	GetCounter(id ContainerIdLike) *LoroCounter
	GetCursorPos(cursor *Cursor) (PosQueryResult, error)
	// Get the entire state of the current DocState
	GetDeepValue() LoroValue
	// Get the entire state of the current DocState with container id
	GetDeepValueWithId() LoroValue
	// Get a [LoroList] by container id.
	//
	// If the provided id is string, it will be converted into a root container id with the name of the string.
	GetList(id ContainerIdLike) *LoroList
	// Get a [LoroMap] by container id.
	//
	// If the provided id is string, it will be converted into a root container id with the name of the string.
	GetMap(id ContainerIdLike) *LoroMap
	// Get a [LoroMovableList] by container id.
	//
	// If the provided id is string, it will be converted into a root container id with the name of the string.
	GetMovableList(id ContainerIdLike) *LoroMovableList
	// Get the path from the root to the container
	GetPathToContainer(id ContainerId) *[]ContainerPath
	// Get the number of operations in the pending transaction.
	//
	// The pending transaction is the one that is not committed yet. It will be committed
	// after calling `doc.commit()`, `doc.export(mode)` or `doc.checkout(version)`.
	GetPendingTxnLen() uint32
	// Get a [LoroText] by container id.
	//
	// If the provided id is string, it will be converted into a root container id with the name of the string.
	GetText(id ContainerIdLike) *LoroText
	// Get a [LoroTree] by container id.
	//
	// If the provided id is string, it will be converted into a root container id with the name of the string.
	GetTree(id ContainerIdLike) *LoroTree
	// Get the shallow value of the document.
	GetValue() LoroValue
	// Check if the doc contains the target container.
	//
	// A root container always exists, while a normal container exists
	// if it has ever been created on the doc.
	HasContainer(id ContainerId) bool
	HasHistoryCache() bool
	// Import updates/snapshot exported by [`LoroDoc::export_snapshot`] or [`LoroDoc::export_from`].
	Import(bytes []byte) (ImportStatus, error)
	// Import a batch of updates/snapshot.
	//
	// The data can be in arbitrary order. The import result will be the same.
	ImportBatch(bytes [][]byte) (ImportStatus, error)
	ImportJsonUpdates(json string) (ImportStatus, error)
	// Import updates/snapshot exported by [`LoroDoc::export_snapshot`] or [`LoroDoc::export_from`].
	//
	// It marks the import with a custom `origin` string. It can be used to track the import source
	// in the generated events.
	ImportWith(bytes []byte, origin string) (ImportStatus, error)
	// Whether the document is in detached mode, where the [loro_internal::DocState] is not
	// synchronized with the latest version of the [loro_internal::OpLog].
	IsDetached() bool
	// Check if the doc contains the full history.
	IsShallow() bool
	// Evaluate a JSONPath expression on the document and return matching values or handlers.
	//
	// This method allows querying the document structure using JSONPath syntax.
	// It returns a vector of `ValueOrHandler` which can represent either primitive values
	// or container handlers, depending on what the JSONPath expression matches.
	//
	// # Arguments
	//
	// * `path` - A string slice containing the JSONPath expression to evaluate.
	//
	// # Returns
	//
	// A `Result` containing either:
	// - `Ok(Vec<ValueOrHandler>)`: A vector of matching values or handlers.
	// - `Err(String)`: An error message if the JSONPath expression is invalid or evaluation fails.
	//
	// # Example
	//
	// ```
	// # use loro::LoroDoc;
	// let doc = LoroDoc::new();
	// let map = doc.get_map("users");
	// map.insert("alice", 30).unwrap();
	// map.insert("bob", 25).unwrap();
	//
	// let result = doc.jsonpath("$.users.alice").unwrap();
	// assert_eq!(result.len(), 1);
	// assert_eq!(result[0].to_json_value(), serde_json::json!(30));
	// ```
	Jsonpath(path string) ([]*ValueOrContainer, error)
	// Get the total number of changes in the `OpLog`
	LenChanges() uint64
	// Get the total number of operations in the `OpLog`
	LenOps() uint64
	// Estimate the size of the document states in memory.
	LogEstimateSize()
	// Minimize the frontiers by removing the unnecessary entries.
	MinimizeFrontiers(frontiers *Frontiers) FrontiersOrId
	// Get the `Frontiers` version of `OpLog`
	OplogFrontiers() *Frontiers
	// Get the `VersionVector` version of `OpLog`
	OplogVv() *VersionVector
	// Get the PeerID
	PeerId() uint64
	// Revert the current document state back to the target version
	//
	// Internally, it will generate a series of local operations that can revert the
	// current doc to the target version. It will calculate the diff between the current
	// state and the target state, and apply the diff to the current state.
	RevertTo(version *Frontiers) error
	// Set the interval of mergeable changes, **in seconds**.
	//
	// If two continuous local changes are within the interval, they will be merged into one change.
	// The default value is 1000 seconds.
	//
	// By default, we record timestamps in seconds for each change. So if the merge interval is 1, and changes A and B
	// have timestamps of 3 and 4 respectively, then they will be merged into one change
	SetChangeMergeInterval(interval int64)
	// Set whether to hide empty root containers.
	SetHideEmptyRootContainers(hide bool)
	// Set commit message for the current uncommitted changes
	//
	// It will be persisted.
	SetNextCommitMessage(msg string)
	// Set the options of the next commit.
	//
	// It will be used when the next commit is performed.
	SetNextCommitOptions(options CommitOptions)
	// Set `origin` for the current uncommitted changes, it can be used to track the source of changes in an event.
	//
	// It will NOT be persisted.
	SetNextCommitOrigin(origin string)
	// Set the timestamp of the next commit.
	//
	// It will be persisted and stored in the `OpLog`.
	// You can get the timestamp from the [`Change`] type.
	SetNextCommitTimestamp(timestamp int64)
	// Change the PeerID
	//
	// NOTE: You need to make sure there is no chance two peer have the same PeerID.
	// If it happens, the document will be corrupted.
	SetPeerId(peer uint64) error
	// Set whether to record the timestamp of each change. Default is `false`.
	//
	// If enabled, the Unix timestamp will be recorded for each change automatically.
	//
	// You can set each timestamp manually when committing a change.
	//
	// NOTE: Timestamps are forced to be in ascending order.
	// If you commit a new change with a timestamp that is less than the existing one,
	// the largest existing timestamp will be used instead.
	SetRecordTimestamp(record bool)
	// Get the `VersionVector` of trimmed history
	//
	// The ops included by the trimmed history are not in the doc.
	ShallowSinceVv() *VersionVector
	// Get the `Frontiers` version of `DocState`
	//
	// Learn more about [`Frontiers`](https://loro.dev/docs/advanced/version_deep_dive)
	StateFrontiers() *Frontiers
	// Get the `VersionVector` version of `DocState`
	StateVv() *VersionVector
	// Subscribe the events of a container.
	//
	// The callback will be invoked when the container is changed.
	// Returns a subscription that can be used to unsubscribe.
	//
	// The events will be emitted after a transaction is committed. A transaction is committed when:
	//
	// - `doc.commit()` is called.
	// - `doc.export(mode)` is called.
	// - `doc.import(data)` is called.
	// - `doc.checkout(version)` is called.
	Subscribe(containerId ContainerId, subscriber Subscriber) *Subscription
	// Subscribe to the first commit from a peer. Operations performed on the `LoroDoc` within this callback
	// will be merged into the current commit.
	//
	// This is useful for managing the relationship between `PeerID` and user information.
	// For example, you could store user names in a `LoroMap` using `PeerID` as the key and the `UserID` as the value.
	SubscribeFirstCommitFromPeer(callback FirstCommitFromPeerCallback) *Subscription
	// Subscribe the local update of the document.
	SubscribeLocalUpdate(callback LocalUpdateCallback) *Subscription
	// Subscribe to the pre-commit event.
	//
	// The callback will be called when the changes are committed but not yet applied to the OpLog.
	// You can modify the commit message and timestamp in the callback by [`ChangeModifier`].
	SubscribePreCommit(callback PreCommitCallback) *Subscription
	// Subscribe all the events.
	//
	// The callback will be invoked when any part of the [loro_internal::DocState] is changed.
	// Returns a subscription that can be used to unsubscribe.
	SubscribeRoot(subscriber Subscriber) *Subscription
	// Traverses the ancestors of the Change containing the given ID, including itself.
	//
	// This method visits all ancestors in causal order, from the latest to the oldest,
	// based on their Lamport timestamps.
	//
	// # Arguments
	//
	// * `ids` - The IDs of the Change to start the traversal from.
	// * `f` - A mutable function that is called for each ancestor. It can return `ControlFlow::Break(())` to stop the traversal.
	TravelChangeAncestors(ids []Id, f ChangeAncestorsTraveler) error
	// Convert `VersionVector` into `Frontiers`
	VvToFrontiers(vv *VersionVector) *Frontiers
}

// `LoroDoc` is the entry for the whole document.
// When it's dropped, all the associated [`Handler`]s will be invalidated.
//
// **Important:** Loro is a pure library and does not handle network protocols.
// It is the responsibility of the user to manage the storage, loading, and synchronization
// of the bytes exported by Loro in a manner suitable for their specific environment.
type LoroDoc struct {
	ffiObject FfiObject
}

// Create a new `LoroDoc` instance.
func NewLoroDoc() *LoroDoc {
	return FfiConverterLoroDocINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_constructor_lorodoc_new(_uniffiStatus)
	}))
}

// Apply a diff to the current document state.
//
// Internally, it will apply the diff to the current state.
func (_self *LoroDoc) ApplyDiff(diff *DiffBatch) error {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorodoc_apply_diff(
			_pointer, FfiConverterDiffBatchINSTANCE.Lower(diff), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Attach the document state to the latest known version.
//
// > The document becomes detached during a `checkout` operation.
// > Being `detached` implies that the `DocState` is not synchronized with the latest version of the `OpLog`.
// > In a detached state, the document is not editable, and any `import` operations will be
// > recorded in the `OpLog` without being applied to the `DocState`.
func (_self *LoroDoc) Attach() {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorodoc_attach(
			_pointer, _uniffiStatus)
		return false
	})
}

// Check the correctness of the document state by comparing it with the state
// calculated by applying all the history.
func (_self *LoroDoc) CheckStateCorrectnessSlow() {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorodoc_check_state_correctness_slow(
			_pointer, _uniffiStatus)
		return false
	})
}

// Checkout the `DocState` to a specific version.
//
// The document becomes detached during a `checkout` operation.
// Being `detached` implies that the `DocState` is not synchronized with the latest version of the `OpLog`.
// In a detached state, the document is not editable, and any `import` operations will be
// recorded in the `OpLog` without being applied to the `DocState`.
//
// You should call `attach` to attach the `DocState` to the latest version of `OpLog`.
func (_self *LoroDoc) Checkout(frontiers *Frontiers) error {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorodoc_checkout(
			_pointer, FfiConverterFrontiersINSTANCE.Lower(frontiers), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Checkout the `DocState` to the latest version.
//
// > The document becomes detached during a `checkout` operation.
// > Being `detached` implies that the `DocState` is not synchronized with the latest version of the `OpLog`.
// > In a detached state, the document is not editable, and any `import` operations will be
// > recorded in the `OpLog` without being applied to the `DocState`.
//
// This has the same effect as `attach`.
func (_self *LoroDoc) CheckoutToLatest() {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorodoc_checkout_to_latest(
			_pointer, _uniffiStatus)
		return false
	})
}

// Clear the options of the next commit.
func (_self *LoroDoc) ClearNextCommitOptions() {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorodoc_clear_next_commit_options(
			_pointer, _uniffiStatus)
		return false
	})
}

// Compare the frontiers with the current OpLog's version.
//
// If `other` contains any version that's not contained in the current OpLog, return [Ordering::Less].
func (_self *LoroDoc) CmpWithFrontiers(other *Frontiers) Ordering {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOrderingINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorodoc_cmp_with_frontiers(
				_pointer, FfiConverterFrontiersINSTANCE.Lower(other), _uniffiStatus),
		}
	}))
}

// Commit the cumulative auto commit transaction.
//
// There is a transaction behind every operation.
// The events will be emitted after a transaction is committed. A transaction is committed when:
//
// - `doc.commit()` is called.
// - `doc.export(mode)` is called.
// - `doc.import(data)` is called.
// - `doc.checkout(version)` is called.
func (_self *LoroDoc) Commit() {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorodoc_commit(
			_pointer, _uniffiStatus)
		return false
	})
}

func (_self *LoroDoc) CommitWith(options CommitOptions) {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorodoc_commit_with(
			_pointer, FfiConverterCommitOptionsINSTANCE.Lower(options), _uniffiStatus)
		return false
	})
}

// Encoded all ops and history cache to bytes and store them in the kv store.
//
// The parsed ops will be dropped
func (_self *LoroDoc) CompactChangeStore() {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorodoc_compact_change_store(
			_pointer, _uniffiStatus)
		return false
	})
}

// Get the configurations of the document.
func (_self *LoroDoc) Config() *Configure {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterConfigureINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_lorodoc_config(
			_pointer, _uniffiStatus)
	}))
}

// Configures the default text style for the document.
//
// This method sets the default text style configuration for the document when using LoroText.
// If `None` is provided, the default style is reset.
//
// # Parameters
//
// - `text_style`: The style configuration to set as the default. `None` to reset.
func (_self *LoroDoc) ConfigDefaultTextStyle(textStyle *StyleConfig) {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorodoc_config_default_text_style(
			_pointer, FfiConverterOptionalStyleConfigINSTANCE.Lower(textStyle), _uniffiStatus)
		return false
	})
}

// Set the rich text format configuration of the document.
//
// You need to config it if you use rich text `mark` method.
// Specifically, you need to config the `expand` property of each style.
//
// Expand is used to specify the behavior of expanding when new text is inserted at the
// beginning or end of the style.
func (_self *LoroDoc) ConfigTextStyle(textStyle *StyleConfigMap) {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorodoc_config_text_style(
			_pointer, FfiConverterStyleConfigMapINSTANCE.Lower(textStyle), _uniffiStatus)
		return false
	})
}

// Delete all content from a root container and hide it from the document.
//
// When a root container is empty and hidden:
// - It won't show up in `get_deep_value()` results
// - It won't be included in document snapshots
//
// Only works on root containers (containers without parents).
func (_self *LoroDoc) DeleteRootContainer(cid ContainerId) {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorodoc_delete_root_container(
			_pointer, FfiConverterContainerIdINSTANCE.Lower(cid), _uniffiStatus)
		return false
	})
}

// Force the document enter the detached mode.
//
// In this mode, when you importing new updates, the [loro_internal::DocState] will not be changed.
//
// Learn more at https://loro.dev/docs/advanced/doc_state_and_oplog#attacheddetached-status
func (_self *LoroDoc) Detach() {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorodoc_detach(
			_pointer, _uniffiStatus)
		return false
	})
}

// Calculate the diff between two versions
func (_self *LoroDoc) Diff(a *Frontiers, b *Frontiers) (*DiffBatch, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_lorodoc_diff(
			_pointer, FfiConverterFrontiersINSTANCE.Lower(a), FfiConverterFrontiersINSTANCE.Lower(b), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *DiffBatch
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterDiffBatchINSTANCE.Lift(_uniffiRV), nil
	}
}

// Exports changes within the specified ID span to JSON schema format.
//
// The JSON schema format produced by this method is identical to the one generated by `export_json_updates`.
// It ensures deterministic output, making it ideal for hash calculations and integrity checks.
//
// This method can also export pending changes from the uncommitted transaction that have not yet been applied to the OpLog.
//
// This method will NOT trigger a new commit implicitly.
func (_self *LoroDoc) ExportJsonInIdSpan(idSpan IdSpan) []string {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorodoc_export_json_in_id_span(
				_pointer, FfiConverterIdSpanINSTANCE.Lower(idSpan), _uniffiStatus),
		}
	}))
}

// Export the current state with json-string format of the document.
func (_self *LoroDoc) ExportJsonUpdates(startVv *VersionVector, endVv *VersionVector) string {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorodoc_export_json_updates(
				_pointer, FfiConverterVersionVectorINSTANCE.Lower(startVv), FfiConverterVersionVectorINSTANCE.Lower(endVv), _uniffiStatus),
		}
	}))
}

// Export the current state with json-string format of the document, without peer compression.
//
// Compared to [`export_json_updates`], this method does not compress the peer IDs in the updates.
// So the operations are easier to be processed by application code.
func (_self *LoroDoc) ExportJsonUpdatesWithoutPeerCompression(startVv *VersionVector, endVv *VersionVector) string {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorodoc_export_json_updates_without_peer_compression(
				_pointer, FfiConverterVersionVectorINSTANCE.Lower(startVv), FfiConverterVersionVectorINSTANCE.Lower(endVv), _uniffiStatus),
		}
	}))
}

func (_self *LoroDoc) ExportShallowSnapshot(frontiers *Frontiers) ([]byte, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroEncodeError](FfiConverterLoroEncodeError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorodoc_export_shallow_snapshot(
				_pointer, FfiConverterFrontiersINSTANCE.Lower(frontiers), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []byte
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBytesINSTANCE.Lift(_uniffiRV), nil
	}
}

// Export the current state and history of the document.
func (_self *LoroDoc) ExportSnapshot() ([]byte, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroEncodeError](FfiConverterLoroEncodeError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorodoc_export_snapshot(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []byte
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBytesINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LoroDoc) ExportSnapshotAt(frontiers *Frontiers) ([]byte, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroEncodeError](FfiConverterLoroEncodeError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorodoc_export_snapshot_at(
				_pointer, FfiConverterFrontiersINSTANCE.Lower(frontiers), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []byte
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBytesINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LoroDoc) ExportStateOnly(frontiers **Frontiers) ([]byte, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroEncodeError](FfiConverterLoroEncodeError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorodoc_export_state_only(
				_pointer, FfiConverterOptionalFrontiersINSTANCE.Lower(frontiers), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []byte
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBytesINSTANCE.Lift(_uniffiRV), nil
	}
}

// Export all the ops not included in the given `VersionVector`
func (_self *LoroDoc) ExportUpdates(vv *VersionVector) ([]byte, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroEncodeError](FfiConverterLoroEncodeError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorodoc_export_updates(
				_pointer, FfiConverterVersionVectorINSTANCE.Lower(vv), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []byte
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBytesINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LoroDoc) ExportUpdatesInRange(spans []IdSpan) ([]byte, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroEncodeError](FfiConverterLoroEncodeError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorodoc_export_updates_in_range(
				_pointer, FfiConverterSequenceIdSpanINSTANCE.Lower(spans), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []byte
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBytesINSTANCE.Lift(_uniffiRV), nil
	}
}

// Find the operation id spans that between the `from` version and the `to` version.
func (_self *LoroDoc) FindIdSpansBetween(from *Frontiers, to *Frontiers) VersionVectorDiff {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterVersionVectorDiffINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorodoc_find_id_spans_between(
				_pointer, FfiConverterFrontiersINSTANCE.Lower(from), FfiConverterFrontiersINSTANCE.Lower(to), _uniffiStatus),
		}
	}))
}

// Duplicate the document with a different PeerID
//
// The time complexity and space complexity of this operation are both O(n),
//
// When called in detached mode, it will fork at the current state frontiers.
// It will have the same effect as `fork_at(&self.state_frontiers())`.
func (_self *LoroDoc) Fork() *LoroDoc {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterLoroDocINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_lorodoc_fork(
			_pointer, _uniffiStatus)
	}))
}

// Fork the document at the given frontiers.
//
// The created doc will only contain the history before the specified frontiers.
func (_self *LoroDoc) ForkAt(frontiers *Frontiers) *LoroDoc {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterLoroDocINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_lorodoc_fork_at(
			_pointer, FfiConverterFrontiersINSTANCE.Lower(frontiers), _uniffiStatus)
	}))
}

// Free the cached diff calculator that is used for checkout.
func (_self *LoroDoc) FreeDiffCalculator() {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorodoc_free_diff_calculator(
			_pointer, _uniffiStatus)
		return false
	})
}

// Free the history cache that is used for making checkout faster.
//
// If you use checkout that switching to an old/concurrent version, the history cache will be built.
// You can free it by calling this method.
func (_self *LoroDoc) FreeHistoryCache() {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorodoc_free_history_cache(
			_pointer, _uniffiStatus)
		return false
	})
}

// Convert `Frontiers` into `VersionVector`
func (_self *LoroDoc) FrontiersToVv(frontiers *Frontiers) **VersionVector {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalVersionVectorINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorodoc_frontiers_to_vv(
				_pointer, FfiConverterFrontiersINSTANCE.Lower(frontiers), _uniffiStatus),
		}
	}))
}

// Get the handler by the path.
func (_self *LoroDoc) GetByPath(path []Index) **ValueOrContainer {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalValueOrContainerINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorodoc_get_by_path(
				_pointer, FfiConverterSequenceIndexINSTANCE.Lower(path), _uniffiStatus),
		}
	}))
}

// The path can be specified in different ways depending on the container type:
//
// For Tree:
// 1. Using node IDs: `tree/{node_id}/property`
// 2. Using indices: `tree/0/1/property`
//
// For List and MovableList:
// - Using indices: `list/0` or `list/1/property`
//
// For Map:
// - Using keys: `map/key` or `map/nested/property`
//
// For tree structures, index-based paths follow depth-first traversal order.
// The indices start from 0 and represent the position of a node among its siblings.
//
// # Examples
// ```
// # use loro::{LoroDoc, LoroValue};
// let doc = LoroDoc::new();
//
// // Tree example
// let tree = doc.get_tree("tree");
// let root = tree.create(None).unwrap();
// tree.get_meta(root).unwrap().insert("name", "root").unwrap();
// // Access tree by ID or index
// let name1 = doc.get_by_str_path(&format!("tree/{}/name", root)).unwrap().into_value().unwrap();
// let name2 = doc.get_by_str_path("tree/0/name").unwrap().into_value().unwrap();
// assert_eq!(name1, name2);
//
// // List example
// let list = doc.get_list("list");
// list.insert(0, "first").unwrap();
// list.insert(1, "second").unwrap();
// // Access list by index
// let item = doc.get_by_str_path("list/0");
// assert_eq!(item.unwrap().into_value().unwrap().into_string().unwrap(), "first".into());
//
// // Map example
// let map = doc.get_map("map");
// map.insert("key", "value").unwrap();
// // Access map by key
// let value = doc.get_by_str_path("map/key");
// assert_eq!(value.unwrap().into_value().unwrap().into_string().unwrap(), "value".into());
//
// // MovableList example
// let mlist = doc.get_movable_list("mlist");
// mlist.insert(0, "item").unwrap();
// // Access movable list by index
// let item = doc.get_by_str_path("mlist/0");
// assert_eq!(item.unwrap().into_value().unwrap().into_string().unwrap(), "item".into());
// ```
func (_self *LoroDoc) GetByStrPath(path string) **ValueOrContainer {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalValueOrContainerINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorodoc_get_by_str_path(
				_pointer, FfiConverterStringINSTANCE.Lower(path), _uniffiStatus),
		}
	}))
}

// Get `Change` at the given id.
//
// `Change` is a grouped continuous operations that share the same id, timestamp, commit message.
//
// - The id of the `Change` is the id of its first op.
// - The second op's id is `{ peer: change.id.peer, counter: change.id.counter + 1 }`
//
// The same applies on `Lamport`:
//
// - The lamport of the `Change` is the lamport of its first op.
// - The second op's lamport is `change.lamport + 1`
//
// The length of the `Change` is how many operations it contains
func (_self *LoroDoc) GetChange(id Id) *ChangeMeta {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalChangeMetaINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorodoc_get_change(
				_pointer, FfiConverterIdINSTANCE.Lower(id), _uniffiStatus),
		}
	}))
}

// Gets container IDs modified in the given ID range.
//
// **NOTE:** This method will implicitly commit.
//
// This method can be used in conjunction with `doc.travel_change_ancestors()` to traverse
// the history and identify all changes that affected specific containers.
//
// # Arguments
//
// * `id` - The starting ID of the change range
// * `len` - The length of the change range to check
func (_self *LoroDoc) GetChangedContainersIn(id Id, len uint32) []ContainerId {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceContainerIdINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorodoc_get_changed_containers_in(
				_pointer, FfiConverterIdINSTANCE.Lower(id), FfiConverterUint32INSTANCE.Lower(len), _uniffiStatus),
		}
	}))
}

// Get a [LoroCounter] by container id.
//
// If the provided id is string, it will be converted into a root container id with the name of the string.
func (_self *LoroDoc) GetCounter(id ContainerIdLike) *LoroCounter {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterLoroCounterINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_lorodoc_get_counter(
			_pointer, FfiConverterContainerIdLikeINSTANCE.Lower(id), _uniffiStatus)
	}))
}

func (_self *LoroDoc) GetCursorPos(cursor *Cursor) (PosQueryResult, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[CannotFindRelativePosition](FfiConverterCannotFindRelativePosition{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorodoc_get_cursor_pos(
				_pointer, FfiConverterCursorINSTANCE.Lower(cursor), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue PosQueryResult
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterPosQueryResultINSTANCE.Lift(_uniffiRV), nil
	}
}

// Get the entire state of the current DocState
func (_self *LoroDoc) GetDeepValue() LoroValue {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterLoroValueINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorodoc_get_deep_value(
				_pointer, _uniffiStatus),
		}
	}))
}

// Get the entire state of the current DocState with container id
func (_self *LoroDoc) GetDeepValueWithId() LoroValue {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterLoroValueINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorodoc_get_deep_value_with_id(
				_pointer, _uniffiStatus),
		}
	}))
}

// Get a [LoroList] by container id.
//
// If the provided id is string, it will be converted into a root container id with the name of the string.
func (_self *LoroDoc) GetList(id ContainerIdLike) *LoroList {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterLoroListINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_lorodoc_get_list(
			_pointer, FfiConverterContainerIdLikeINSTANCE.Lower(id), _uniffiStatus)
	}))
}

// Get a [LoroMap] by container id.
//
// If the provided id is string, it will be converted into a root container id with the name of the string.
func (_self *LoroDoc) GetMap(id ContainerIdLike) *LoroMap {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterLoroMapINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_lorodoc_get_map(
			_pointer, FfiConverterContainerIdLikeINSTANCE.Lower(id), _uniffiStatus)
	}))
}

// Get a [LoroMovableList] by container id.
//
// If the provided id is string, it will be converted into a root container id with the name of the string.
func (_self *LoroDoc) GetMovableList(id ContainerIdLike) *LoroMovableList {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterLoroMovableListINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_lorodoc_get_movable_list(
			_pointer, FfiConverterContainerIdLikeINSTANCE.Lower(id), _uniffiStatus)
	}))
}

// Get the path from the root to the container
func (_self *LoroDoc) GetPathToContainer(id ContainerId) *[]ContainerPath {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalSequenceContainerPathINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorodoc_get_path_to_container(
				_pointer, FfiConverterContainerIdINSTANCE.Lower(id), _uniffiStatus),
		}
	}))
}

// Get the number of operations in the pending transaction.
//
// The pending transaction is the one that is not committed yet. It will be committed
// after calling `doc.commit()`, `doc.export(mode)` or `doc.checkout(version)`.
func (_self *LoroDoc) GetPendingTxnLen() uint32 {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint32INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.uniffi_loro_fn_method_lorodoc_get_pending_txn_len(
			_pointer, _uniffiStatus)
	}))
}

// Get a [LoroText] by container id.
//
// If the provided id is string, it will be converted into a root container id with the name of the string.
func (_self *LoroDoc) GetText(id ContainerIdLike) *LoroText {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterLoroTextINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_lorodoc_get_text(
			_pointer, FfiConverterContainerIdLikeINSTANCE.Lower(id), _uniffiStatus)
	}))
}

// Get a [LoroTree] by container id.
//
// If the provided id is string, it will be converted into a root container id with the name of the string.
func (_self *LoroDoc) GetTree(id ContainerIdLike) *LoroTree {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterLoroTreeINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_lorodoc_get_tree(
			_pointer, FfiConverterContainerIdLikeINSTANCE.Lower(id), _uniffiStatus)
	}))
}

// Get the shallow value of the document.
func (_self *LoroDoc) GetValue() LoroValue {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterLoroValueINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorodoc_get_value(
				_pointer, _uniffiStatus),
		}
	}))
}

// Check if the doc contains the target container.
//
// A root container always exists, while a normal container exists
// if it has ever been created on the doc.
func (_self *LoroDoc) HasContainer(id ContainerId) bool {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_loro_fn_method_lorodoc_has_container(
			_pointer, FfiConverterContainerIdINSTANCE.Lower(id), _uniffiStatus)
	}))
}

func (_self *LoroDoc) HasHistoryCache() bool {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_loro_fn_method_lorodoc_has_history_cache(
			_pointer, _uniffiStatus)
	}))
}

// Import updates/snapshot exported by [`LoroDoc::export_snapshot`] or [`LoroDoc::export_from`].
func (_self *LoroDoc) Import(bytes []byte) (ImportStatus, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorodoc_import(
				_pointer, FfiConverterBytesINSTANCE.Lower(bytes), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue ImportStatus
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterImportStatusINSTANCE.Lift(_uniffiRV), nil
	}
}

// Import a batch of updates/snapshot.
//
// The data can be in arbitrary order. The import result will be the same.
func (_self *LoroDoc) ImportBatch(bytes [][]byte) (ImportStatus, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorodoc_import_batch(
				_pointer, FfiConverterSequenceBytesINSTANCE.Lower(bytes), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue ImportStatus
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterImportStatusINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LoroDoc) ImportJsonUpdates(json string) (ImportStatus, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorodoc_import_json_updates(
				_pointer, FfiConverterStringINSTANCE.Lower(json), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue ImportStatus
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterImportStatusINSTANCE.Lift(_uniffiRV), nil
	}
}

// Import updates/snapshot exported by [`LoroDoc::export_snapshot`] or [`LoroDoc::export_from`].
//
// It marks the import with a custom `origin` string. It can be used to track the import source
// in the generated events.
func (_self *LoroDoc) ImportWith(bytes []byte, origin string) (ImportStatus, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorodoc_import_with(
				_pointer, FfiConverterBytesINSTANCE.Lower(bytes), FfiConverterStringINSTANCE.Lower(origin), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue ImportStatus
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterImportStatusINSTANCE.Lift(_uniffiRV), nil
	}
}

// Whether the document is in detached mode, where the [loro_internal::DocState] is not
// synchronized with the latest version of the [loro_internal::OpLog].
func (_self *LoroDoc) IsDetached() bool {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_loro_fn_method_lorodoc_is_detached(
			_pointer, _uniffiStatus)
	}))
}

// Check if the doc contains the full history.
func (_self *LoroDoc) IsShallow() bool {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_loro_fn_method_lorodoc_is_shallow(
			_pointer, _uniffiStatus)
	}))
}

// Evaluate a JSONPath expression on the document and return matching values or handlers.
//
// This method allows querying the document structure using JSONPath syntax.
// It returns a vector of `ValueOrHandler` which can represent either primitive values
// or container handlers, depending on what the JSONPath expression matches.
//
// # Arguments
//
// * `path` - A string slice containing the JSONPath expression to evaluate.
//
// # Returns
//
// A `Result` containing either:
// - `Ok(Vec<ValueOrHandler>)`: A vector of matching values or handlers.
// - `Err(String)`: An error message if the JSONPath expression is invalid or evaluation fails.
//
// # Example
//
// ```
// # use loro::LoroDoc;
// let doc = LoroDoc::new();
// let map = doc.get_map("users");
// map.insert("alice", 30).unwrap();
// map.insert("bob", 25).unwrap();
//
// let result = doc.jsonpath("$.users.alice").unwrap();
// assert_eq!(result.len(), 1);
// assert_eq!(result[0].to_json_value(), serde_json::json!(30));
// ```
func (_self *LoroDoc) Jsonpath(path string) ([]*ValueOrContainer, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[JsonPathError](FfiConverterJsonPathError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorodoc_jsonpath(
				_pointer, FfiConverterStringINSTANCE.Lower(path), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []*ValueOrContainer
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSequenceValueOrContainerINSTANCE.Lift(_uniffiRV), nil
	}
}

// Get the total number of changes in the `OpLog`
func (_self *LoroDoc) LenChanges() uint64 {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint64_t {
		return C.uniffi_loro_fn_method_lorodoc_len_changes(
			_pointer, _uniffiStatus)
	}))
}

// Get the total number of operations in the `OpLog`
func (_self *LoroDoc) LenOps() uint64 {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint64_t {
		return C.uniffi_loro_fn_method_lorodoc_len_ops(
			_pointer, _uniffiStatus)
	}))
}

// Estimate the size of the document states in memory.
func (_self *LoroDoc) LogEstimateSize() {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorodoc_log_estimate_size(
			_pointer, _uniffiStatus)
		return false
	})
}

// Minimize the frontiers by removing the unnecessary entries.
func (_self *LoroDoc) MinimizeFrontiers(frontiers *Frontiers) FrontiersOrId {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterFrontiersOrIdINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorodoc_minimize_frontiers(
				_pointer, FfiConverterFrontiersINSTANCE.Lower(frontiers), _uniffiStatus),
		}
	}))
}

// Get the `Frontiers` version of `OpLog`
func (_self *LoroDoc) OplogFrontiers() *Frontiers {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterFrontiersINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_lorodoc_oplog_frontiers(
			_pointer, _uniffiStatus)
	}))
}

// Get the `VersionVector` version of `OpLog`
func (_self *LoroDoc) OplogVv() *VersionVector {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterVersionVectorINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_lorodoc_oplog_vv(
			_pointer, _uniffiStatus)
	}))
}

// Get the PeerID
func (_self *LoroDoc) PeerId() uint64 {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint64_t {
		return C.uniffi_loro_fn_method_lorodoc_peer_id(
			_pointer, _uniffiStatus)
	}))
}

// Revert the current document state back to the target version
//
// Internally, it will generate a series of local operations that can revert the
// current doc to the target version. It will calculate the diff between the current
// state and the target state, and apply the diff to the current state.
func (_self *LoroDoc) RevertTo(version *Frontiers) error {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorodoc_revert_to(
			_pointer, FfiConverterFrontiersINSTANCE.Lower(version), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Set the interval of mergeable changes, **in seconds**.
//
// If two continuous local changes are within the interval, they will be merged into one change.
// The default value is 1000 seconds.
//
// By default, we record timestamps in seconds for each change. So if the merge interval is 1, and changes A and B
// have timestamps of 3 and 4 respectively, then they will be merged into one change
func (_self *LoroDoc) SetChangeMergeInterval(interval int64) {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorodoc_set_change_merge_interval(
			_pointer, FfiConverterInt64INSTANCE.Lower(interval), _uniffiStatus)
		return false
	})
}

// Set whether to hide empty root containers.
func (_self *LoroDoc) SetHideEmptyRootContainers(hide bool) {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorodoc_set_hide_empty_root_containers(
			_pointer, FfiConverterBoolINSTANCE.Lower(hide), _uniffiStatus)
		return false
	})
}

// Set commit message for the current uncommitted changes
//
// It will be persisted.
func (_self *LoroDoc) SetNextCommitMessage(msg string) {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorodoc_set_next_commit_message(
			_pointer, FfiConverterStringINSTANCE.Lower(msg), _uniffiStatus)
		return false
	})
}

// Set the options of the next commit.
//
// It will be used when the next commit is performed.
func (_self *LoroDoc) SetNextCommitOptions(options CommitOptions) {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorodoc_set_next_commit_options(
			_pointer, FfiConverterCommitOptionsINSTANCE.Lower(options), _uniffiStatus)
		return false
	})
}

// Set `origin` for the current uncommitted changes, it can be used to track the source of changes in an event.
//
// It will NOT be persisted.
func (_self *LoroDoc) SetNextCommitOrigin(origin string) {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorodoc_set_next_commit_origin(
			_pointer, FfiConverterStringINSTANCE.Lower(origin), _uniffiStatus)
		return false
	})
}

// Set the timestamp of the next commit.
//
// It will be persisted and stored in the `OpLog`.
// You can get the timestamp from the [`Change`] type.
func (_self *LoroDoc) SetNextCommitTimestamp(timestamp int64) {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorodoc_set_next_commit_timestamp(
			_pointer, FfiConverterInt64INSTANCE.Lower(timestamp), _uniffiStatus)
		return false
	})
}

// Change the PeerID
//
// NOTE: You need to make sure there is no chance two peer have the same PeerID.
// If it happens, the document will be corrupted.
func (_self *LoroDoc) SetPeerId(peer uint64) error {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorodoc_set_peer_id(
			_pointer, FfiConverterUint64INSTANCE.Lower(peer), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Set whether to record the timestamp of each change. Default is `false`.
//
// If enabled, the Unix timestamp will be recorded for each change automatically.
//
// You can set each timestamp manually when committing a change.
//
// NOTE: Timestamps are forced to be in ascending order.
// If you commit a new change with a timestamp that is less than the existing one,
// the largest existing timestamp will be used instead.
func (_self *LoroDoc) SetRecordTimestamp(record bool) {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorodoc_set_record_timestamp(
			_pointer, FfiConverterBoolINSTANCE.Lower(record), _uniffiStatus)
		return false
	})
}

// Get the `VersionVector` of trimmed history
//
// The ops included by the trimmed history are not in the doc.
func (_self *LoroDoc) ShallowSinceVv() *VersionVector {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterVersionVectorINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_lorodoc_shallow_since_vv(
			_pointer, _uniffiStatus)
	}))
}

// Get the `Frontiers` version of `DocState`
//
// Learn more about [`Frontiers`](https://loro.dev/docs/advanced/version_deep_dive)
func (_self *LoroDoc) StateFrontiers() *Frontiers {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterFrontiersINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_lorodoc_state_frontiers(
			_pointer, _uniffiStatus)
	}))
}

// Get the `VersionVector` version of `DocState`
func (_self *LoroDoc) StateVv() *VersionVector {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterVersionVectorINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_lorodoc_state_vv(
			_pointer, _uniffiStatus)
	}))
}

// Subscribe the events of a container.
//
// The callback will be invoked when the container is changed.
// Returns a subscription that can be used to unsubscribe.
//
// The events will be emitted after a transaction is committed. A transaction is committed when:
//
// - `doc.commit()` is called.
// - `doc.export(mode)` is called.
// - `doc.import(data)` is called.
// - `doc.checkout(version)` is called.
func (_self *LoroDoc) Subscribe(containerId ContainerId, subscriber Subscriber) *Subscription {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSubscriptionINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_lorodoc_subscribe(
			_pointer, FfiConverterContainerIdINSTANCE.Lower(containerId), FfiConverterSubscriberINSTANCE.Lower(subscriber), _uniffiStatus)
	}))
}

// Subscribe to the first commit from a peer. Operations performed on the `LoroDoc` within this callback
// will be merged into the current commit.
//
// This is useful for managing the relationship between `PeerID` and user information.
// For example, you could store user names in a `LoroMap` using `PeerID` as the key and the `UserID` as the value.
func (_self *LoroDoc) SubscribeFirstCommitFromPeer(callback FirstCommitFromPeerCallback) *Subscription {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSubscriptionINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_lorodoc_subscribe_first_commit_from_peer(
			_pointer, FfiConverterFirstCommitFromPeerCallbackINSTANCE.Lower(callback), _uniffiStatus)
	}))
}

// Subscribe the local update of the document.
func (_self *LoroDoc) SubscribeLocalUpdate(callback LocalUpdateCallback) *Subscription {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSubscriptionINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_lorodoc_subscribe_local_update(
			_pointer, FfiConverterLocalUpdateCallbackINSTANCE.Lower(callback), _uniffiStatus)
	}))
}

// Subscribe to the pre-commit event.
//
// The callback will be called when the changes are committed but not yet applied to the OpLog.
// You can modify the commit message and timestamp in the callback by [`ChangeModifier`].
func (_self *LoroDoc) SubscribePreCommit(callback PreCommitCallback) *Subscription {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSubscriptionINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_lorodoc_subscribe_pre_commit(
			_pointer, FfiConverterPreCommitCallbackINSTANCE.Lower(callback), _uniffiStatus)
	}))
}

// Subscribe all the events.
//
// The callback will be invoked when any part of the [loro_internal::DocState] is changed.
// Returns a subscription that can be used to unsubscribe.
func (_self *LoroDoc) SubscribeRoot(subscriber Subscriber) *Subscription {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSubscriptionINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_lorodoc_subscribe_root(
			_pointer, FfiConverterSubscriberINSTANCE.Lower(subscriber), _uniffiStatus)
	}))
}

// Traverses the ancestors of the Change containing the given ID, including itself.
//
// This method visits all ancestors in causal order, from the latest to the oldest,
// based on their Lamport timestamps.
//
// # Arguments
//
// * `ids` - The IDs of the Change to start the traversal from.
// * `f` - A mutable function that is called for each ancestor. It can return `ControlFlow::Break(())` to stop the traversal.
func (_self *LoroDoc) TravelChangeAncestors(ids []Id, f ChangeAncestorsTraveler) error {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[ChangeTravelError](FfiConverterChangeTravelError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorodoc_travel_change_ancestors(
			_pointer, FfiConverterSequenceIdINSTANCE.Lower(ids), FfiConverterChangeAncestorsTravelerINSTANCE.Lower(f), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Convert `VersionVector` into `Frontiers`
func (_self *LoroDoc) VvToFrontiers(vv *VersionVector) *Frontiers {
	_pointer := _self.ffiObject.incrementPointer("*LoroDoc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterFrontiersINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_lorodoc_vv_to_frontiers(
			_pointer, FfiConverterVersionVectorINSTANCE.Lower(vv), _uniffiStatus)
	}))
}
func (object *LoroDoc) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterLoroDoc struct{}

var FfiConverterLoroDocINSTANCE = FfiConverterLoroDoc{}

func (c FfiConverterLoroDoc) Lift(pointer unsafe.Pointer) *LoroDoc {
	result := &LoroDoc{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_loro_fn_clone_lorodoc(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_loro_fn_free_lorodoc(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*LoroDoc).Destroy)
	return result
}

func (c FfiConverterLoroDoc) Read(reader io.Reader) *LoroDoc {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterLoroDoc) Lower(value *LoroDoc) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*LoroDoc")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterLoroDoc) Write(writer io.Writer, value *LoroDoc) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerLoroDoc struct{}

func (_ FfiDestroyerLoroDoc) Destroy(value *LoroDoc) {
	value.Destroy()
}

type LoroListInterface interface {
	// Delete all elements in the list.
	Clear() error
	// Delete values at the given position.
	Delete(pos uint32, len uint32) error
	// Get the LoroDoc from this container
	Doc() **LoroDoc
	// Get the value at the given position.
	Get(index uint32) **ValueOrContainer
	// If a detached container is attached, this method will return its corresponding attached handler.
	GetAttached() **LoroList
	GetCursor(pos uint32, side Side) **Cursor
	// Get the deep value of the container.
	GetDeepValue() LoroValue
	// Get the ID of the list item at the given position.
	GetIdAt(pos uint32) *Id
	// Get the shallow value of the container.
	//
	// This does not convert the state of sub-containers; instead, it represents them as [LoroValue::Container].
	GetValue() LoroValue
	// Get the ID of the container.
	Id() ContainerId
	// Insert a value at the given position.
	Insert(pos uint32, v LoroValueLike) error
	InsertCounterContainer(pos uint32, child *LoroCounter) (*LoroCounter, error)
	InsertListContainer(pos uint32, child *LoroList) (*LoroList, error)
	InsertMapContainer(pos uint32, child *LoroMap) (*LoroMap, error)
	InsertMovableListContainer(pos uint32, child *LoroMovableList) (*LoroMovableList, error)
	InsertTextContainer(pos uint32, child *LoroText) (*LoroText, error)
	InsertTreeContainer(pos uint32, child *LoroTree) (*LoroTree, error)
	// Whether the container is attached to a document
	//
	// The edits on a detached container will not be persisted.
	// To attach the container to the document, please insert it into an attached container.
	IsAttached() bool
	// Whether the container is deleted.
	IsDeleted() bool
	IsEmpty() bool
	Len() uint32
	// Pop the last element of the list.
	Pop() (*LoroValue, error)
	Push(v LoroValueLike) error
	// Converts the LoroList to a Vec of LoroValue.
	//
	// This method unwraps the internal Arc and clones the data if necessary,
	// returning a Vec containing all the elements of the LoroList as LoroValue.
	ToVec() []LoroValue
}
type LoroList struct {
	ffiObject FfiObject
}

// Create a new container that is detached from the document.
//
// The edits on a detached container will not be persisted.
// To attach the container to the document, please insert it into an attached container.
func NewLoroList() *LoroList {
	return FfiConverterLoroListINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_constructor_lorolist_new(_uniffiStatus)
	}))
}

// Delete all elements in the list.
func (_self *LoroList) Clear() error {
	_pointer := _self.ffiObject.incrementPointer("*LoroList")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorolist_clear(
			_pointer, _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Delete values at the given position.
func (_self *LoroList) Delete(pos uint32, len uint32) error {
	_pointer := _self.ffiObject.incrementPointer("*LoroList")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorolist_delete(
			_pointer, FfiConverterUint32INSTANCE.Lower(pos), FfiConverterUint32INSTANCE.Lower(len), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Get the LoroDoc from this container
func (_self *LoroList) Doc() **LoroDoc {
	_pointer := _self.ffiObject.incrementPointer("*LoroList")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalLoroDocINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorolist_doc(
				_pointer, _uniffiStatus),
		}
	}))
}

// Get the value at the given position.
func (_self *LoroList) Get(index uint32) **ValueOrContainer {
	_pointer := _self.ffiObject.incrementPointer("*LoroList")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalValueOrContainerINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorolist_get(
				_pointer, FfiConverterUint32INSTANCE.Lower(index), _uniffiStatus),
		}
	}))
}

// If a detached container is attached, this method will return its corresponding attached handler.
func (_self *LoroList) GetAttached() **LoroList {
	_pointer := _self.ffiObject.incrementPointer("*LoroList")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalLoroListINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorolist_get_attached(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *LoroList) GetCursor(pos uint32, side Side) **Cursor {
	_pointer := _self.ffiObject.incrementPointer("*LoroList")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalCursorINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorolist_get_cursor(
				_pointer, FfiConverterUint32INSTANCE.Lower(pos), FfiConverterSideINSTANCE.Lower(side), _uniffiStatus),
		}
	}))
}

// Get the deep value of the container.
func (_self *LoroList) GetDeepValue() LoroValue {
	_pointer := _self.ffiObject.incrementPointer("*LoroList")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterLoroValueINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorolist_get_deep_value(
				_pointer, _uniffiStatus),
		}
	}))
}

// Get the ID of the list item at the given position.
func (_self *LoroList) GetIdAt(pos uint32) *Id {
	_pointer := _self.ffiObject.incrementPointer("*LoroList")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalIdINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorolist_get_id_at(
				_pointer, FfiConverterUint32INSTANCE.Lower(pos), _uniffiStatus),
		}
	}))
}

// Get the shallow value of the container.
//
// This does not convert the state of sub-containers; instead, it represents them as [LoroValue::Container].
func (_self *LoroList) GetValue() LoroValue {
	_pointer := _self.ffiObject.incrementPointer("*LoroList")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterLoroValueINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorolist_get_value(
				_pointer, _uniffiStatus),
		}
	}))
}

// Get the ID of the container.
func (_self *LoroList) Id() ContainerId {
	_pointer := _self.ffiObject.incrementPointer("*LoroList")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterContainerIdINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorolist_id(
				_pointer, _uniffiStatus),
		}
	}))
}

// Insert a value at the given position.
func (_self *LoroList) Insert(pos uint32, v LoroValueLike) error {
	_pointer := _self.ffiObject.incrementPointer("*LoroList")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorolist_insert(
			_pointer, FfiConverterUint32INSTANCE.Lower(pos), FfiConverterLoroValueLikeINSTANCE.Lower(v), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

func (_self *LoroList) InsertCounterContainer(pos uint32, child *LoroCounter) (*LoroCounter, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroList")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_lorolist_insert_counter_container(
			_pointer, FfiConverterUint32INSTANCE.Lower(pos), FfiConverterLoroCounterINSTANCE.Lower(child), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LoroCounter
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLoroCounterINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LoroList) InsertListContainer(pos uint32, child *LoroList) (*LoroList, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroList")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_lorolist_insert_list_container(
			_pointer, FfiConverterUint32INSTANCE.Lower(pos), FfiConverterLoroListINSTANCE.Lower(child), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LoroList
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLoroListINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LoroList) InsertMapContainer(pos uint32, child *LoroMap) (*LoroMap, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroList")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_lorolist_insert_map_container(
			_pointer, FfiConverterUint32INSTANCE.Lower(pos), FfiConverterLoroMapINSTANCE.Lower(child), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LoroMap
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLoroMapINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LoroList) InsertMovableListContainer(pos uint32, child *LoroMovableList) (*LoroMovableList, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroList")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_lorolist_insert_movable_list_container(
			_pointer, FfiConverterUint32INSTANCE.Lower(pos), FfiConverterLoroMovableListINSTANCE.Lower(child), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LoroMovableList
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLoroMovableListINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LoroList) InsertTextContainer(pos uint32, child *LoroText) (*LoroText, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroList")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_lorolist_insert_text_container(
			_pointer, FfiConverterUint32INSTANCE.Lower(pos), FfiConverterLoroTextINSTANCE.Lower(child), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LoroText
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLoroTextINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LoroList) InsertTreeContainer(pos uint32, child *LoroTree) (*LoroTree, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroList")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_lorolist_insert_tree_container(
			_pointer, FfiConverterUint32INSTANCE.Lower(pos), FfiConverterLoroTreeINSTANCE.Lower(child), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LoroTree
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLoroTreeINSTANCE.Lift(_uniffiRV), nil
	}
}

// Whether the container is attached to a document
//
// The edits on a detached container will not be persisted.
// To attach the container to the document, please insert it into an attached container.
func (_self *LoroList) IsAttached() bool {
	_pointer := _self.ffiObject.incrementPointer("*LoroList")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_loro_fn_method_lorolist_is_attached(
			_pointer, _uniffiStatus)
	}))
}

// Whether the container is deleted.
func (_self *LoroList) IsDeleted() bool {
	_pointer := _self.ffiObject.incrementPointer("*LoroList")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_loro_fn_method_lorolist_is_deleted(
			_pointer, _uniffiStatus)
	}))
}

func (_self *LoroList) IsEmpty() bool {
	_pointer := _self.ffiObject.incrementPointer("*LoroList")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_loro_fn_method_lorolist_is_empty(
			_pointer, _uniffiStatus)
	}))
}

func (_self *LoroList) Len() uint32 {
	_pointer := _self.ffiObject.incrementPointer("*LoroList")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint32INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.uniffi_loro_fn_method_lorolist_len(
			_pointer, _uniffiStatus)
	}))
}

// Pop the last element of the list.
func (_self *LoroList) Pop() (*LoroValue, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroList")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorolist_pop(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LoroValue
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterOptionalLoroValueINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LoroList) Push(v LoroValueLike) error {
	_pointer := _self.ffiObject.incrementPointer("*LoroList")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorolist_push(
			_pointer, FfiConverterLoroValueLikeINSTANCE.Lower(v), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Converts the LoroList to a Vec of LoroValue.
//
// This method unwraps the internal Arc and clones the data if necessary,
// returning a Vec containing all the elements of the LoroList as LoroValue.
func (_self *LoroList) ToVec() []LoroValue {
	_pointer := _self.ffiObject.incrementPointer("*LoroList")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceLoroValueINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorolist_to_vec(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *LoroList) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterLoroList struct{}

var FfiConverterLoroListINSTANCE = FfiConverterLoroList{}

func (c FfiConverterLoroList) Lift(pointer unsafe.Pointer) *LoroList {
	result := &LoroList{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_loro_fn_clone_lorolist(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_loro_fn_free_lorolist(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*LoroList).Destroy)
	return result
}

func (c FfiConverterLoroList) Read(reader io.Reader) *LoroList {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterLoroList) Lower(value *LoroList) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*LoroList")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterLoroList) Write(writer io.Writer, value *LoroList) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerLoroList struct{}

func (_ FfiDestroyerLoroList) Destroy(value *LoroList) {
	value.Destroy()
}

type LoroMapInterface interface {
	// Delete all key-value pairs in the map.
	Clear() error
	// Delete a key-value pair from the map.
	Delete(key string) error
	// Get the LoroDoc from this container
	Doc() **LoroDoc
	// Get the value of the map with the given key.
	Get(key string) **ValueOrContainer
	// If a detached container is attached, this method will return its corresponding attached handler.
	GetAttached() **LoroMap
	// Get the deep value of the map.
	//
	// It will convert the state of sub-containers into a nested JSON value.
	GetDeepValue() LoroValue
	// Get the peer id of the last editor on the given entry
	GetLastEditor(key string) *uint64
	GetOrCreateCounterContainer(key string, child *LoroCounter) (*LoroCounter, error)
	GetOrCreateListContainer(key string, child *LoroList) (*LoroList, error)
	GetOrCreateMapContainer(key string, child *LoroMap) (*LoroMap, error)
	GetOrCreateMovableListContainer(key string, child *LoroMovableList) (*LoroMovableList, error)
	GetOrCreateTextContainer(key string, child *LoroText) (*LoroText, error)
	GetOrCreateTreeContainer(key string, child *LoroTree) (*LoroTree, error)
	// Get the shallow value of the map.
	//
	// It will not convert the state of sub-containers, but represent them as [LoroValue::Container].
	GetValue() LoroValue
	// Get the ID of the map.
	Id() ContainerId
	// Insert a key-value pair into the map.
	//
	// > **Note**: When calling `map.set(key, value)` on a LoroMap, if `map.get(key)` already returns `value`,
	// > the operation will be a no-op (no operation recorded) to avoid unnecessary updates.
	Insert(key string, v LoroValueLike) error
	InsertCounterContainer(key string, child *LoroCounter) (*LoroCounter, error)
	InsertListContainer(key string, child *LoroList) (*LoroList, error)
	InsertMapContainer(key string, child *LoroMap) (*LoroMap, error)
	InsertMovableListContainer(key string, child *LoroMovableList) (*LoroMovableList, error)
	InsertTextContainer(key string, child *LoroText) (*LoroText, error)
	InsertTreeContainer(key string, child *LoroTree) (*LoroTree, error)
	// Whether the container is attached to a document.
	IsAttached() bool
	// Whether the container is deleted.
	IsDeleted() bool
	// Whether the map is empty.
	IsEmpty() bool
	// Get the keys of the map.
	Keys() []string
	// Get the length of the map.
	Len() uint32
	// Get the values of the map.
	Values() []*ValueOrContainer
}
type LoroMap struct {
	ffiObject FfiObject
}

// Create a new container that is detached from the document.
//
// The edits on a detached container will not be persisted.
// To attach the container to the document, please insert it into an attached container.
func NewLoroMap() *LoroMap {
	return FfiConverterLoroMapINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_constructor_loromap_new(_uniffiStatus)
	}))
}

// Delete all key-value pairs in the map.
func (_self *LoroMap) Clear() error {
	_pointer := _self.ffiObject.incrementPointer("*LoroMap")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_loromap_clear(
			_pointer, _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Delete a key-value pair from the map.
func (_self *LoroMap) Delete(key string) error {
	_pointer := _self.ffiObject.incrementPointer("*LoroMap")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_loromap_delete(
			_pointer, FfiConverterStringINSTANCE.Lower(key), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Get the LoroDoc from this container
func (_self *LoroMap) Doc() **LoroDoc {
	_pointer := _self.ffiObject.incrementPointer("*LoroMap")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalLoroDocINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_loromap_doc(
				_pointer, _uniffiStatus),
		}
	}))
}

// Get the value of the map with the given key.
func (_self *LoroMap) Get(key string) **ValueOrContainer {
	_pointer := _self.ffiObject.incrementPointer("*LoroMap")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalValueOrContainerINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_loromap_get(
				_pointer, FfiConverterStringINSTANCE.Lower(key), _uniffiStatus),
		}
	}))
}

// If a detached container is attached, this method will return its corresponding attached handler.
func (_self *LoroMap) GetAttached() **LoroMap {
	_pointer := _self.ffiObject.incrementPointer("*LoroMap")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalLoroMapINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_loromap_get_attached(
				_pointer, _uniffiStatus),
		}
	}))
}

// Get the deep value of the map.
//
// It will convert the state of sub-containers into a nested JSON value.
func (_self *LoroMap) GetDeepValue() LoroValue {
	_pointer := _self.ffiObject.incrementPointer("*LoroMap")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterLoroValueINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_loromap_get_deep_value(
				_pointer, _uniffiStatus),
		}
	}))
}

// Get the peer id of the last editor on the given entry
func (_self *LoroMap) GetLastEditor(key string) *uint64 {
	_pointer := _self.ffiObject.incrementPointer("*LoroMap")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_loromap_get_last_editor(
				_pointer, FfiConverterStringINSTANCE.Lower(key), _uniffiStatus),
		}
	}))
}

func (_self *LoroMap) GetOrCreateCounterContainer(key string, child *LoroCounter) (*LoroCounter, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroMap")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_loromap_get_or_create_counter_container(
			_pointer, FfiConverterStringINSTANCE.Lower(key), FfiConverterLoroCounterINSTANCE.Lower(child), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LoroCounter
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLoroCounterINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LoroMap) GetOrCreateListContainer(key string, child *LoroList) (*LoroList, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroMap")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_loromap_get_or_create_list_container(
			_pointer, FfiConverterStringINSTANCE.Lower(key), FfiConverterLoroListINSTANCE.Lower(child), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LoroList
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLoroListINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LoroMap) GetOrCreateMapContainer(key string, child *LoroMap) (*LoroMap, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroMap")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_loromap_get_or_create_map_container(
			_pointer, FfiConverterStringINSTANCE.Lower(key), FfiConverterLoroMapINSTANCE.Lower(child), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LoroMap
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLoroMapINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LoroMap) GetOrCreateMovableListContainer(key string, child *LoroMovableList) (*LoroMovableList, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroMap")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_loromap_get_or_create_movable_list_container(
			_pointer, FfiConverterStringINSTANCE.Lower(key), FfiConverterLoroMovableListINSTANCE.Lower(child), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LoroMovableList
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLoroMovableListINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LoroMap) GetOrCreateTextContainer(key string, child *LoroText) (*LoroText, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroMap")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_loromap_get_or_create_text_container(
			_pointer, FfiConverterStringINSTANCE.Lower(key), FfiConverterLoroTextINSTANCE.Lower(child), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LoroText
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLoroTextINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LoroMap) GetOrCreateTreeContainer(key string, child *LoroTree) (*LoroTree, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroMap")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_loromap_get_or_create_tree_container(
			_pointer, FfiConverterStringINSTANCE.Lower(key), FfiConverterLoroTreeINSTANCE.Lower(child), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LoroTree
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLoroTreeINSTANCE.Lift(_uniffiRV), nil
	}
}

// Get the shallow value of the map.
//
// It will not convert the state of sub-containers, but represent them as [LoroValue::Container].
func (_self *LoroMap) GetValue() LoroValue {
	_pointer := _self.ffiObject.incrementPointer("*LoroMap")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterLoroValueINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_loromap_get_value(
				_pointer, _uniffiStatus),
		}
	}))
}

// Get the ID of the map.
func (_self *LoroMap) Id() ContainerId {
	_pointer := _self.ffiObject.incrementPointer("*LoroMap")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterContainerIdINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_loromap_id(
				_pointer, _uniffiStatus),
		}
	}))
}

// Insert a key-value pair into the map.
//
// > **Note**: When calling `map.set(key, value)` on a LoroMap, if `map.get(key)` already returns `value`,
// > the operation will be a no-op (no operation recorded) to avoid unnecessary updates.
func (_self *LoroMap) Insert(key string, v LoroValueLike) error {
	_pointer := _self.ffiObject.incrementPointer("*LoroMap")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_loromap_insert(
			_pointer, FfiConverterStringINSTANCE.Lower(key), FfiConverterLoroValueLikeINSTANCE.Lower(v), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

func (_self *LoroMap) InsertCounterContainer(key string, child *LoroCounter) (*LoroCounter, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroMap")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_loromap_insert_counter_container(
			_pointer, FfiConverterStringINSTANCE.Lower(key), FfiConverterLoroCounterINSTANCE.Lower(child), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LoroCounter
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLoroCounterINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LoroMap) InsertListContainer(key string, child *LoroList) (*LoroList, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroMap")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_loromap_insert_list_container(
			_pointer, FfiConverterStringINSTANCE.Lower(key), FfiConverterLoroListINSTANCE.Lower(child), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LoroList
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLoroListINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LoroMap) InsertMapContainer(key string, child *LoroMap) (*LoroMap, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroMap")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_loromap_insert_map_container(
			_pointer, FfiConverterStringINSTANCE.Lower(key), FfiConverterLoroMapINSTANCE.Lower(child), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LoroMap
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLoroMapINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LoroMap) InsertMovableListContainer(key string, child *LoroMovableList) (*LoroMovableList, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroMap")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_loromap_insert_movable_list_container(
			_pointer, FfiConverterStringINSTANCE.Lower(key), FfiConverterLoroMovableListINSTANCE.Lower(child), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LoroMovableList
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLoroMovableListINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LoroMap) InsertTextContainer(key string, child *LoroText) (*LoroText, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroMap")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_loromap_insert_text_container(
			_pointer, FfiConverterStringINSTANCE.Lower(key), FfiConverterLoroTextINSTANCE.Lower(child), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LoroText
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLoroTextINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LoroMap) InsertTreeContainer(key string, child *LoroTree) (*LoroTree, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroMap")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_loromap_insert_tree_container(
			_pointer, FfiConverterStringINSTANCE.Lower(key), FfiConverterLoroTreeINSTANCE.Lower(child), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LoroTree
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLoroTreeINSTANCE.Lift(_uniffiRV), nil
	}
}

// Whether the container is attached to a document.
func (_self *LoroMap) IsAttached() bool {
	_pointer := _self.ffiObject.incrementPointer("*LoroMap")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_loro_fn_method_loromap_is_attached(
			_pointer, _uniffiStatus)
	}))
}

// Whether the container is deleted.
func (_self *LoroMap) IsDeleted() bool {
	_pointer := _self.ffiObject.incrementPointer("*LoroMap")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_loro_fn_method_loromap_is_deleted(
			_pointer, _uniffiStatus)
	}))
}

// Whether the map is empty.
func (_self *LoroMap) IsEmpty() bool {
	_pointer := _self.ffiObject.incrementPointer("*LoroMap")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_loro_fn_method_loromap_is_empty(
			_pointer, _uniffiStatus)
	}))
}

// Get the keys of the map.
func (_self *LoroMap) Keys() []string {
	_pointer := _self.ffiObject.incrementPointer("*LoroMap")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_loromap_keys(
				_pointer, _uniffiStatus),
		}
	}))
}

// Get the length of the map.
func (_self *LoroMap) Len() uint32 {
	_pointer := _self.ffiObject.incrementPointer("*LoroMap")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint32INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.uniffi_loro_fn_method_loromap_len(
			_pointer, _uniffiStatus)
	}))
}

// Get the values of the map.
func (_self *LoroMap) Values() []*ValueOrContainer {
	_pointer := _self.ffiObject.incrementPointer("*LoroMap")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceValueOrContainerINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_loromap_values(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *LoroMap) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterLoroMap struct{}

var FfiConverterLoroMapINSTANCE = FfiConverterLoroMap{}

func (c FfiConverterLoroMap) Lift(pointer unsafe.Pointer) *LoroMap {
	result := &LoroMap{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_loro_fn_clone_loromap(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_loro_fn_free_loromap(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*LoroMap).Destroy)
	return result
}

func (c FfiConverterLoroMap) Read(reader io.Reader) *LoroMap {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterLoroMap) Lower(value *LoroMap) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*LoroMap")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterLoroMap) Write(writer io.Writer, value *LoroMap) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerLoroMap struct{}

func (_ FfiDestroyerLoroMap) Destroy(value *LoroMap) {
	value.Destroy()
}

type LoroMovableListInterface interface {
	// Delete all elements in the list.
	Clear() error
	// Delete values at the given position.
	Delete(pos uint32, len uint32) error
	// Get the LoroDoc from this container
	Doc() **LoroDoc
	// Get the value at the given position.
	Get(index uint32) **ValueOrContainer
	// If a detached container is attached, this method will return its corresponding attached handler.
	GetAttached() **LoroMovableList
	GetCreatorAt(pos uint32) *uint64
	// Get the cursor at the given position.
	//
	// Using "index" to denote cursor positions can be unstable, as positions may
	// shift with document edits. To reliably represent a position or range within
	// a document, it is more effective to leverage the unique ID of each item/character
	// in a List CRDT or Text CRDT.
	//
	// Loro optimizes State metadata by not storing the IDs of deleted elements. This
	// approach complicates tracking cursors since they rely on these IDs. The solution
	// recalculates position by replaying relevant history to update stable positions
	// accurately. To minimize the performance impact of history replay, the system
	// updates cursor info to reference only the IDs of currently present elements,
	// thereby reducing the need for replay.
	GetCursor(pos uint32, side Side) **Cursor
	// Get the deep value of the container.
	GetDeepValue() LoroValue
	// Get the last editor of the list item at the given position.
	GetLastEditorAt(pos uint32) *uint64
	// Get the last mover of the list item at the given position.
	GetLastMoverAt(pos uint32) *uint64
	// Get the shallow value of the container.
	//
	// This does not convert the state of sub-containers; instead, it represents them as [LoroValue::Container].
	GetValue() LoroValue
	// Get the container id.
	Id() ContainerId
	// Insert a value at the given position.
	Insert(pos uint32, v LoroValueLike) error
	InsertCounterContainer(pos uint32, child *LoroCounter) (*LoroCounter, error)
	InsertListContainer(pos uint32, child *LoroList) (*LoroList, error)
	InsertMapContainer(pos uint32, child *LoroMap) (*LoroMap, error)
	InsertMovableListContainer(pos uint32, child *LoroMovableList) (*LoroMovableList, error)
	InsertTextContainer(pos uint32, child *LoroText) (*LoroText, error)
	InsertTreeContainer(pos uint32, child *LoroTree) (*LoroTree, error)
	// Whether the container is attached to a document
	//
	// The edits on a detached container will not be persisted.
	// To attach the container to the document, please insert it into an attached container.
	IsAttached() bool
	// Whether the container is deleted.
	IsDeleted() bool
	IsEmpty() bool
	Len() uint32
	// Move the value at the given position to the given position.
	Mov(from uint32, to uint32) error
	// Pop the last element of the list.
	Pop() (**ValueOrContainer, error)
	Push(v LoroValueLike) error
	// Set the value at the given position.
	Set(pos uint32, value LoroValueLike) error
	SetCounterContainer(pos uint32, child *LoroCounter) (*LoroCounter, error)
	SetListContainer(pos uint32, child *LoroList) (*LoroList, error)
	SetMapContainer(pos uint32, child *LoroMap) (*LoroMap, error)
	SetMovableListContainer(pos uint32, child *LoroMovableList) (*LoroMovableList, error)
	SetTextContainer(pos uint32, child *LoroText) (*LoroText, error)
	SetTreeContainer(pos uint32, child *LoroTree) (*LoroTree, error)
	// Get the elements of the list as a vector of LoroValues.
	//
	// This method returns a vector containing all the elements in the list as LoroValues.
	// It provides a convenient way to access the entire contents of the LoroMovableList
	// as a standard Rust vector.
	ToVec() []LoroValue
}
type LoroMovableList struct {
	ffiObject FfiObject
}

// Create a new container that is detached from the document.
//
// The edits on a detached container will not be persisted.
// To attach the container to the document, please insert it into an attached container.
func NewLoroMovableList() *LoroMovableList {
	return FfiConverterLoroMovableListINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_constructor_loromovablelist_new(_uniffiStatus)
	}))
}

// Delete all elements in the list.
func (_self *LoroMovableList) Clear() error {
	_pointer := _self.ffiObject.incrementPointer("*LoroMovableList")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_loromovablelist_clear(
			_pointer, _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Delete values at the given position.
func (_self *LoroMovableList) Delete(pos uint32, len uint32) error {
	_pointer := _self.ffiObject.incrementPointer("*LoroMovableList")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_loromovablelist_delete(
			_pointer, FfiConverterUint32INSTANCE.Lower(pos), FfiConverterUint32INSTANCE.Lower(len), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Get the LoroDoc from this container
func (_self *LoroMovableList) Doc() **LoroDoc {
	_pointer := _self.ffiObject.incrementPointer("*LoroMovableList")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalLoroDocINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_loromovablelist_doc(
				_pointer, _uniffiStatus),
		}
	}))
}

// Get the value at the given position.
func (_self *LoroMovableList) Get(index uint32) **ValueOrContainer {
	_pointer := _self.ffiObject.incrementPointer("*LoroMovableList")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalValueOrContainerINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_loromovablelist_get(
				_pointer, FfiConverterUint32INSTANCE.Lower(index), _uniffiStatus),
		}
	}))
}

// If a detached container is attached, this method will return its corresponding attached handler.
func (_self *LoroMovableList) GetAttached() **LoroMovableList {
	_pointer := _self.ffiObject.incrementPointer("*LoroMovableList")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalLoroMovableListINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_loromovablelist_get_attached(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *LoroMovableList) GetCreatorAt(pos uint32) *uint64 {
	_pointer := _self.ffiObject.incrementPointer("*LoroMovableList")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_loromovablelist_get_creator_at(
				_pointer, FfiConverterUint32INSTANCE.Lower(pos), _uniffiStatus),
		}
	}))
}

// Get the cursor at the given position.
//
// Using "index" to denote cursor positions can be unstable, as positions may
// shift with document edits. To reliably represent a position or range within
// a document, it is more effective to leverage the unique ID of each item/character
// in a List CRDT or Text CRDT.
//
// Loro optimizes State metadata by not storing the IDs of deleted elements. This
// approach complicates tracking cursors since they rely on these IDs. The solution
// recalculates position by replaying relevant history to update stable positions
// accurately. To minimize the performance impact of history replay, the system
// updates cursor info to reference only the IDs of currently present elements,
// thereby reducing the need for replay.
func (_self *LoroMovableList) GetCursor(pos uint32, side Side) **Cursor {
	_pointer := _self.ffiObject.incrementPointer("*LoroMovableList")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalCursorINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_loromovablelist_get_cursor(
				_pointer, FfiConverterUint32INSTANCE.Lower(pos), FfiConverterSideINSTANCE.Lower(side), _uniffiStatus),
		}
	}))
}

// Get the deep value of the container.
func (_self *LoroMovableList) GetDeepValue() LoroValue {
	_pointer := _self.ffiObject.incrementPointer("*LoroMovableList")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterLoroValueINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_loromovablelist_get_deep_value(
				_pointer, _uniffiStatus),
		}
	}))
}

// Get the last editor of the list item at the given position.
func (_self *LoroMovableList) GetLastEditorAt(pos uint32) *uint64 {
	_pointer := _self.ffiObject.incrementPointer("*LoroMovableList")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_loromovablelist_get_last_editor_at(
				_pointer, FfiConverterUint32INSTANCE.Lower(pos), _uniffiStatus),
		}
	}))
}

// Get the last mover of the list item at the given position.
func (_self *LoroMovableList) GetLastMoverAt(pos uint32) *uint64 {
	_pointer := _self.ffiObject.incrementPointer("*LoroMovableList")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_loromovablelist_get_last_mover_at(
				_pointer, FfiConverterUint32INSTANCE.Lower(pos), _uniffiStatus),
		}
	}))
}

// Get the shallow value of the container.
//
// This does not convert the state of sub-containers; instead, it represents them as [LoroValue::Container].
func (_self *LoroMovableList) GetValue() LoroValue {
	_pointer := _self.ffiObject.incrementPointer("*LoroMovableList")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterLoroValueINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_loromovablelist_get_value(
				_pointer, _uniffiStatus),
		}
	}))
}

// Get the container id.
func (_self *LoroMovableList) Id() ContainerId {
	_pointer := _self.ffiObject.incrementPointer("*LoroMovableList")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterContainerIdINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_loromovablelist_id(
				_pointer, _uniffiStatus),
		}
	}))
}

// Insert a value at the given position.
func (_self *LoroMovableList) Insert(pos uint32, v LoroValueLike) error {
	_pointer := _self.ffiObject.incrementPointer("*LoroMovableList")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_loromovablelist_insert(
			_pointer, FfiConverterUint32INSTANCE.Lower(pos), FfiConverterLoroValueLikeINSTANCE.Lower(v), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

func (_self *LoroMovableList) InsertCounterContainer(pos uint32, child *LoroCounter) (*LoroCounter, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroMovableList")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_loromovablelist_insert_counter_container(
			_pointer, FfiConverterUint32INSTANCE.Lower(pos), FfiConverterLoroCounterINSTANCE.Lower(child), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LoroCounter
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLoroCounterINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LoroMovableList) InsertListContainer(pos uint32, child *LoroList) (*LoroList, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroMovableList")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_loromovablelist_insert_list_container(
			_pointer, FfiConverterUint32INSTANCE.Lower(pos), FfiConverterLoroListINSTANCE.Lower(child), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LoroList
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLoroListINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LoroMovableList) InsertMapContainer(pos uint32, child *LoroMap) (*LoroMap, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroMovableList")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_loromovablelist_insert_map_container(
			_pointer, FfiConverterUint32INSTANCE.Lower(pos), FfiConverterLoroMapINSTANCE.Lower(child), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LoroMap
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLoroMapINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LoroMovableList) InsertMovableListContainer(pos uint32, child *LoroMovableList) (*LoroMovableList, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroMovableList")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_loromovablelist_insert_movable_list_container(
			_pointer, FfiConverterUint32INSTANCE.Lower(pos), FfiConverterLoroMovableListINSTANCE.Lower(child), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LoroMovableList
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLoroMovableListINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LoroMovableList) InsertTextContainer(pos uint32, child *LoroText) (*LoroText, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroMovableList")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_loromovablelist_insert_text_container(
			_pointer, FfiConverterUint32INSTANCE.Lower(pos), FfiConverterLoroTextINSTANCE.Lower(child), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LoroText
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLoroTextINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LoroMovableList) InsertTreeContainer(pos uint32, child *LoroTree) (*LoroTree, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroMovableList")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_loromovablelist_insert_tree_container(
			_pointer, FfiConverterUint32INSTANCE.Lower(pos), FfiConverterLoroTreeINSTANCE.Lower(child), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LoroTree
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLoroTreeINSTANCE.Lift(_uniffiRV), nil
	}
}

// Whether the container is attached to a document
//
// The edits on a detached container will not be persisted.
// To attach the container to the document, please insert it into an attached container.
func (_self *LoroMovableList) IsAttached() bool {
	_pointer := _self.ffiObject.incrementPointer("*LoroMovableList")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_loro_fn_method_loromovablelist_is_attached(
			_pointer, _uniffiStatus)
	}))
}

// Whether the container is deleted.
func (_self *LoroMovableList) IsDeleted() bool {
	_pointer := _self.ffiObject.incrementPointer("*LoroMovableList")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_loro_fn_method_loromovablelist_is_deleted(
			_pointer, _uniffiStatus)
	}))
}

func (_self *LoroMovableList) IsEmpty() bool {
	_pointer := _self.ffiObject.incrementPointer("*LoroMovableList")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_loro_fn_method_loromovablelist_is_empty(
			_pointer, _uniffiStatus)
	}))
}

func (_self *LoroMovableList) Len() uint32 {
	_pointer := _self.ffiObject.incrementPointer("*LoroMovableList")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint32INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.uniffi_loro_fn_method_loromovablelist_len(
			_pointer, _uniffiStatus)
	}))
}

// Move the value at the given position to the given position.
func (_self *LoroMovableList) Mov(from uint32, to uint32) error {
	_pointer := _self.ffiObject.incrementPointer("*LoroMovableList")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_loromovablelist_mov(
			_pointer, FfiConverterUint32INSTANCE.Lower(from), FfiConverterUint32INSTANCE.Lower(to), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Pop the last element of the list.
func (_self *LoroMovableList) Pop() (**ValueOrContainer, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroMovableList")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_loromovablelist_pop(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue **ValueOrContainer
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterOptionalValueOrContainerINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LoroMovableList) Push(v LoroValueLike) error {
	_pointer := _self.ffiObject.incrementPointer("*LoroMovableList")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_loromovablelist_push(
			_pointer, FfiConverterLoroValueLikeINSTANCE.Lower(v), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Set the value at the given position.
func (_self *LoroMovableList) Set(pos uint32, value LoroValueLike) error {
	_pointer := _self.ffiObject.incrementPointer("*LoroMovableList")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_loromovablelist_set(
			_pointer, FfiConverterUint32INSTANCE.Lower(pos), FfiConverterLoroValueLikeINSTANCE.Lower(value), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

func (_self *LoroMovableList) SetCounterContainer(pos uint32, child *LoroCounter) (*LoroCounter, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroMovableList")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_loromovablelist_set_counter_container(
			_pointer, FfiConverterUint32INSTANCE.Lower(pos), FfiConverterLoroCounterINSTANCE.Lower(child), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LoroCounter
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLoroCounterINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LoroMovableList) SetListContainer(pos uint32, child *LoroList) (*LoroList, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroMovableList")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_loromovablelist_set_list_container(
			_pointer, FfiConverterUint32INSTANCE.Lower(pos), FfiConverterLoroListINSTANCE.Lower(child), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LoroList
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLoroListINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LoroMovableList) SetMapContainer(pos uint32, child *LoroMap) (*LoroMap, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroMovableList")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_loromovablelist_set_map_container(
			_pointer, FfiConverterUint32INSTANCE.Lower(pos), FfiConverterLoroMapINSTANCE.Lower(child), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LoroMap
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLoroMapINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LoroMovableList) SetMovableListContainer(pos uint32, child *LoroMovableList) (*LoroMovableList, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroMovableList")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_loromovablelist_set_movable_list_container(
			_pointer, FfiConverterUint32INSTANCE.Lower(pos), FfiConverterLoroMovableListINSTANCE.Lower(child), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LoroMovableList
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLoroMovableListINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LoroMovableList) SetTextContainer(pos uint32, child *LoroText) (*LoroText, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroMovableList")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_loromovablelist_set_text_container(
			_pointer, FfiConverterUint32INSTANCE.Lower(pos), FfiConverterLoroTextINSTANCE.Lower(child), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LoroText
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLoroTextINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LoroMovableList) SetTreeContainer(pos uint32, child *LoroTree) (*LoroTree, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroMovableList")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_loromovablelist_set_tree_container(
			_pointer, FfiConverterUint32INSTANCE.Lower(pos), FfiConverterLoroTreeINSTANCE.Lower(child), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LoroTree
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLoroTreeINSTANCE.Lift(_uniffiRV), nil
	}
}

// Get the elements of the list as a vector of LoroValues.
//
// This method returns a vector containing all the elements in the list as LoroValues.
// It provides a convenient way to access the entire contents of the LoroMovableList
// as a standard Rust vector.
func (_self *LoroMovableList) ToVec() []LoroValue {
	_pointer := _self.ffiObject.incrementPointer("*LoroMovableList")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceLoroValueINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_loromovablelist_to_vec(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *LoroMovableList) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterLoroMovableList struct{}

var FfiConverterLoroMovableListINSTANCE = FfiConverterLoroMovableList{}

func (c FfiConverterLoroMovableList) Lift(pointer unsafe.Pointer) *LoroMovableList {
	result := &LoroMovableList{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_loro_fn_clone_loromovablelist(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_loro_fn_free_loromovablelist(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*LoroMovableList).Destroy)
	return result
}

func (c FfiConverterLoroMovableList) Read(reader io.Reader) *LoroMovableList {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterLoroMovableList) Lower(value *LoroMovableList) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*LoroMovableList")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterLoroMovableList) Write(writer io.Writer, value *LoroMovableList) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerLoroMovableList struct{}

func (_ FfiDestroyerLoroMovableList) Destroy(value *LoroMovableList) {
	value.Destroy()
}

type LoroTextInterface interface {
	// Apply a [delta](https://quilljs.com/docs/delta/) to the text container.
	ApplyDelta(delta []TextDelta) error
	// Delete a range of text at the given unicode position with unicode length.
	Delete(pos uint32, len uint32) error
	// Delete a range of text at the given utf-8 position with utf-8 length.
	DeleteUtf8(pos uint32, len uint32) error
	// Get the LoroDoc from this container
	Doc() **LoroDoc
	// If a detached container is attached, this method will return its corresponding attached handler.
	GetAttached() **LoroText
	// Get the cursor at the given position in the given Unicode position..
	//
	// Using "index" to denote cursor positions can be unstable, as positions may
	// shift with document edits. To reliably represent a position or range within
	// a document, it is more effective to leverage the unique ID of each item/character
	// in a List CRDT or Text CRDT.
	//
	// Loro optimizes State metadata by not storing the IDs of deleted elements. This
	// approach complicates tracking cursors since they rely on these IDs. The solution
	// recalculates position by replaying relevant history to update stable positions
	// accurately. To minimize the performance impact of history replay, the system
	// updates cursor info to reference only the IDs of currently present elements,
	// thereby reducing the need for replay.
	GetCursor(pos uint32, side Side) **Cursor
	// Get the editor of the text at the given position.
	GetEditorAtUnicodePos(pos uint32) *uint64
	// Get the text in [Delta](https://quilljs.com/docs/delta/) format.
	GetRichtextValue() LoroValue
	// Get the [ContainerID]  of the text container.
	Id() ContainerId
	// Insert a string at the given unicode position.
	Insert(pos uint32, s string) error
	// Insert a string at the given utf-8 position.
	InsertUtf8(pos uint32, s string) error
	// Whether the container is attached to a document
	//
	// The edits on a detached container will not be persisted.
	// To attach the container to the document, please insert it into an attached container.
	IsAttached() bool
	// Whether the container is deleted.
	IsDeleted() bool
	// Whether the text container is empty.
	IsEmpty() bool
	// Get the length of the text container in Unicode.
	LenUnicode() uint32
	// Get the length of the text container in UTF-16.
	LenUtf16() uint32
	// Get the length of the text container in UTF-8.
	LenUtf8() uint32
	// Mark a range of text with a key-value pair.
	//
	// You can use it to create a highlight, make a range of text bold, or add a link to a range of text.
	//
	// You can specify the `expand` option to set the behavior when inserting text at the boundary of the range.
	//
	// - `after`(default): when inserting text right after the given range, the mark will be expanded to include the inserted text
	// - `before`: when inserting text right before the given range, the mark will be expanded to include the inserted text
	// - `none`: the mark will not be expanded to include the inserted text at the boundaries
	// - `both`: when inserting text either right before or right after the given range, the mark will be expanded to include the inserted text
	//
	// *You should make sure that a key is always associated with the same expand type.*
	//
	// Note: this is not suitable for unmergeable annotations like comments.
	Mark(from uint32, to uint32, key string, value LoroValueLike) error
	// Push a string to the end of the text container.
	PushStr(s string) error
	// Get a string slice at the given Unicode range
	Slice(startIndex uint32, endIndex uint32) (string, error)
	// Delete specified character and insert string at the same position at given unicode position.
	Splice(pos uint32, len uint32, s string) (string, error)
	// Get the text in [Delta](https://quilljs.com/docs/delta/) format.
	ToDelta() []TextDelta
	// Get the text content of the text container.
	ToString() string
	// Unmark a range of text with a key and a value.
	//
	// You can use it to remove highlights, bolds or links
	//
	// You can specify the `expand` option to set the behavior when inserting text at the boundary of the range.
	//
	// **Note: You should specify the same expand type as when you mark the text.**
	//
	// - `after`(default): when inserting text right after the given range, the mark will be expanded to include the inserted text
	// - `before`: when inserting text right before the given range, the mark will be expanded to include the inserted text
	// - `none`: the mark will not be expanded to include the inserted text at the boundaries
	// - `both`: when inserting text either right before or right after the given range, the mark will be expanded to include the inserted text
	//
	// *You should make sure that a key is always associated with the same expand type.*
	//
	// Note: you cannot delete unmergeable annotations like comments by this method.
	Unmark(from uint32, to uint32, key string) error
	// Update the current text based on the provided text.
	//
	// It will calculate the minimal difference and apply it to the current text.
	// It uses Myers' diff algorithm to compute the optimal difference.
	//
	// This could take a long time for large texts (e.g. > 50_000 characters).
	// In that case, you should use `updateByLine` instead.
	Update(s string, options UpdateOptions) error
	// Update the current text based on the provided text.
	//
	// This update calculation is line-based, which will be more efficient but less precise.
	UpdateByLine(s string, options UpdateOptions) error
}
type LoroText struct {
	ffiObject FfiObject
}

// Create a new container that is detached from the document.
//
// The edits on a detached container will not be persisted.
// To attach the container to the document, please insert it into an attached container.
func NewLoroText() *LoroText {
	return FfiConverterLoroTextINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_constructor_lorotext_new(_uniffiStatus)
	}))
}

// Apply a [delta](https://quilljs.com/docs/delta/) to the text container.
func (_self *LoroText) ApplyDelta(delta []TextDelta) error {
	_pointer := _self.ffiObject.incrementPointer("*LoroText")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorotext_apply_delta(
			_pointer, FfiConverterSequenceTextDeltaINSTANCE.Lower(delta), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Delete a range of text at the given unicode position with unicode length.
func (_self *LoroText) Delete(pos uint32, len uint32) error {
	_pointer := _self.ffiObject.incrementPointer("*LoroText")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorotext_delete(
			_pointer, FfiConverterUint32INSTANCE.Lower(pos), FfiConverterUint32INSTANCE.Lower(len), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Delete a range of text at the given utf-8 position with utf-8 length.
func (_self *LoroText) DeleteUtf8(pos uint32, len uint32) error {
	_pointer := _self.ffiObject.incrementPointer("*LoroText")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorotext_delete_utf8(
			_pointer, FfiConverterUint32INSTANCE.Lower(pos), FfiConverterUint32INSTANCE.Lower(len), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Get the LoroDoc from this container
func (_self *LoroText) Doc() **LoroDoc {
	_pointer := _self.ffiObject.incrementPointer("*LoroText")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalLoroDocINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorotext_doc(
				_pointer, _uniffiStatus),
		}
	}))
}

// If a detached container is attached, this method will return its corresponding attached handler.
func (_self *LoroText) GetAttached() **LoroText {
	_pointer := _self.ffiObject.incrementPointer("*LoroText")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalLoroTextINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorotext_get_attached(
				_pointer, _uniffiStatus),
		}
	}))
}

// Get the cursor at the given position in the given Unicode position..
//
// Using "index" to denote cursor positions can be unstable, as positions may
// shift with document edits. To reliably represent a position or range within
// a document, it is more effective to leverage the unique ID of each item/character
// in a List CRDT or Text CRDT.
//
// Loro optimizes State metadata by not storing the IDs of deleted elements. This
// approach complicates tracking cursors since they rely on these IDs. The solution
// recalculates position by replaying relevant history to update stable positions
// accurately. To minimize the performance impact of history replay, the system
// updates cursor info to reference only the IDs of currently present elements,
// thereby reducing the need for replay.
func (_self *LoroText) GetCursor(pos uint32, side Side) **Cursor {
	_pointer := _self.ffiObject.incrementPointer("*LoroText")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalCursorINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorotext_get_cursor(
				_pointer, FfiConverterUint32INSTANCE.Lower(pos), FfiConverterSideINSTANCE.Lower(side), _uniffiStatus),
		}
	}))
}

// Get the editor of the text at the given position.
func (_self *LoroText) GetEditorAtUnicodePos(pos uint32) *uint64 {
	_pointer := _self.ffiObject.incrementPointer("*LoroText")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorotext_get_editor_at_unicode_pos(
				_pointer, FfiConverterUint32INSTANCE.Lower(pos), _uniffiStatus),
		}
	}))
}

// Get the text in [Delta](https://quilljs.com/docs/delta/) format.
func (_self *LoroText) GetRichtextValue() LoroValue {
	_pointer := _self.ffiObject.incrementPointer("*LoroText")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterLoroValueINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorotext_get_richtext_value(
				_pointer, _uniffiStatus),
		}
	}))
}

// Get the [ContainerID]  of the text container.
func (_self *LoroText) Id() ContainerId {
	_pointer := _self.ffiObject.incrementPointer("*LoroText")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterContainerIdINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorotext_id(
				_pointer, _uniffiStatus),
		}
	}))
}

// Insert a string at the given unicode position.
func (_self *LoroText) Insert(pos uint32, s string) error {
	_pointer := _self.ffiObject.incrementPointer("*LoroText")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorotext_insert(
			_pointer, FfiConverterUint32INSTANCE.Lower(pos), FfiConverterStringINSTANCE.Lower(s), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Insert a string at the given utf-8 position.
func (_self *LoroText) InsertUtf8(pos uint32, s string) error {
	_pointer := _self.ffiObject.incrementPointer("*LoroText")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorotext_insert_utf8(
			_pointer, FfiConverterUint32INSTANCE.Lower(pos), FfiConverterStringINSTANCE.Lower(s), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Whether the container is attached to a document
//
// The edits on a detached container will not be persisted.
// To attach the container to the document, please insert it into an attached container.
func (_self *LoroText) IsAttached() bool {
	_pointer := _self.ffiObject.incrementPointer("*LoroText")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_loro_fn_method_lorotext_is_attached(
			_pointer, _uniffiStatus)
	}))
}

// Whether the container is deleted.
func (_self *LoroText) IsDeleted() bool {
	_pointer := _self.ffiObject.incrementPointer("*LoroText")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_loro_fn_method_lorotext_is_deleted(
			_pointer, _uniffiStatus)
	}))
}

// Whether the text container is empty.
func (_self *LoroText) IsEmpty() bool {
	_pointer := _self.ffiObject.incrementPointer("*LoroText")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_loro_fn_method_lorotext_is_empty(
			_pointer, _uniffiStatus)
	}))
}

// Get the length of the text container in Unicode.
func (_self *LoroText) LenUnicode() uint32 {
	_pointer := _self.ffiObject.incrementPointer("*LoroText")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint32INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.uniffi_loro_fn_method_lorotext_len_unicode(
			_pointer, _uniffiStatus)
	}))
}

// Get the length of the text container in UTF-16.
func (_self *LoroText) LenUtf16() uint32 {
	_pointer := _self.ffiObject.incrementPointer("*LoroText")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint32INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.uniffi_loro_fn_method_lorotext_len_utf16(
			_pointer, _uniffiStatus)
	}))
}

// Get the length of the text container in UTF-8.
func (_self *LoroText) LenUtf8() uint32 {
	_pointer := _self.ffiObject.incrementPointer("*LoroText")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint32INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.uniffi_loro_fn_method_lorotext_len_utf8(
			_pointer, _uniffiStatus)
	}))
}

// Mark a range of text with a key-value pair.
//
// You can use it to create a highlight, make a range of text bold, or add a link to a range of text.
//
// You can specify the `expand` option to set the behavior when inserting text at the boundary of the range.
//
// - `after`(default): when inserting text right after the given range, the mark will be expanded to include the inserted text
// - `before`: when inserting text right before the given range, the mark will be expanded to include the inserted text
// - `none`: the mark will not be expanded to include the inserted text at the boundaries
// - `both`: when inserting text either right before or right after the given range, the mark will be expanded to include the inserted text
//
// *You should make sure that a key is always associated with the same expand type.*
//
// Note: this is not suitable for unmergeable annotations like comments.
func (_self *LoroText) Mark(from uint32, to uint32, key string, value LoroValueLike) error {
	_pointer := _self.ffiObject.incrementPointer("*LoroText")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorotext_mark(
			_pointer, FfiConverterUint32INSTANCE.Lower(from), FfiConverterUint32INSTANCE.Lower(to), FfiConverterStringINSTANCE.Lower(key), FfiConverterLoroValueLikeINSTANCE.Lower(value), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Push a string to the end of the text container.
func (_self *LoroText) PushStr(s string) error {
	_pointer := _self.ffiObject.incrementPointer("*LoroText")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorotext_push_str(
			_pointer, FfiConverterStringINSTANCE.Lower(s), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Get a string slice at the given Unicode range
func (_self *LoroText) Slice(startIndex uint32, endIndex uint32) (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroText")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorotext_slice(
				_pointer, FfiConverterUint32INSTANCE.Lower(startIndex), FfiConverterUint32INSTANCE.Lower(endIndex), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterStringINSTANCE.Lift(_uniffiRV), nil
	}
}

// Delete specified character and insert string at the same position at given unicode position.
func (_self *LoroText) Splice(pos uint32, len uint32, s string) (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroText")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorotext_splice(
				_pointer, FfiConverterUint32INSTANCE.Lower(pos), FfiConverterUint32INSTANCE.Lower(len), FfiConverterStringINSTANCE.Lower(s), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterStringINSTANCE.Lift(_uniffiRV), nil
	}
}

// Get the text in [Delta](https://quilljs.com/docs/delta/) format.
func (_self *LoroText) ToDelta() []TextDelta {
	_pointer := _self.ffiObject.incrementPointer("*LoroText")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceTextDeltaINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorotext_to_delta(
				_pointer, _uniffiStatus),
		}
	}))
}

// Get the text content of the text container.
func (_self *LoroText) ToString() string {
	_pointer := _self.ffiObject.incrementPointer("*LoroText")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorotext_to_string(
				_pointer, _uniffiStatus),
		}
	}))
}

// Unmark a range of text with a key and a value.
//
// # You can use it to remove highlights, bolds or links
//
// You can specify the `expand` option to set the behavior when inserting text at the boundary of the range.
//
// **Note: You should specify the same expand type as when you mark the text.**
//
// - `after`(default): when inserting text right after the given range, the mark will be expanded to include the inserted text
// - `before`: when inserting text right before the given range, the mark will be expanded to include the inserted text
// - `none`: the mark will not be expanded to include the inserted text at the boundaries
// - `both`: when inserting text either right before or right after the given range, the mark will be expanded to include the inserted text
//
// *You should make sure that a key is always associated with the same expand type.*
//
// Note: you cannot delete unmergeable annotations like comments by this method.
func (_self *LoroText) Unmark(from uint32, to uint32, key string) error {
	_pointer := _self.ffiObject.incrementPointer("*LoroText")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorotext_unmark(
			_pointer, FfiConverterUint32INSTANCE.Lower(from), FfiConverterUint32INSTANCE.Lower(to), FfiConverterStringINSTANCE.Lower(key), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Update the current text based on the provided text.
//
// It will calculate the minimal difference and apply it to the current text.
// It uses Myers' diff algorithm to compute the optimal difference.
//
// This could take a long time for large texts (e.g. > 50_000 characters).
// In that case, you should use `updateByLine` instead.
func (_self *LoroText) Update(s string, options UpdateOptions) error {
	_pointer := _self.ffiObject.incrementPointer("*LoroText")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[UpdateTimeoutError](FfiConverterUpdateTimeoutError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorotext_update(
			_pointer, FfiConverterStringINSTANCE.Lower(s), FfiConverterUpdateOptionsINSTANCE.Lower(options), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Update the current text based on the provided text.
//
// This update calculation is line-based, which will be more efficient but less precise.
func (_self *LoroText) UpdateByLine(s string, options UpdateOptions) error {
	_pointer := _self.ffiObject.incrementPointer("*LoroText")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[UpdateTimeoutError](FfiConverterUpdateTimeoutError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorotext_update_by_line(
			_pointer, FfiConverterStringINSTANCE.Lower(s), FfiConverterUpdateOptionsINSTANCE.Lower(options), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}
func (object *LoroText) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterLoroText struct{}

var FfiConverterLoroTextINSTANCE = FfiConverterLoroText{}

func (c FfiConverterLoroText) Lift(pointer unsafe.Pointer) *LoroText {
	result := &LoroText{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_loro_fn_clone_lorotext(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_loro_fn_free_lorotext(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*LoroText).Destroy)
	return result
}

func (c FfiConverterLoroText) Read(reader io.Reader) *LoroText {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterLoroText) Lower(value *LoroText) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*LoroText")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterLoroText) Write(writer io.Writer, value *LoroText) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerLoroText struct{}

func (_ FfiDestroyerLoroText) Destroy(value *LoroText) {
	value.Destroy()
}

type LoroTreeInterface interface {
	// Return all children of the target node.
	//
	// If the parent node does not exist, return `None`.
	Children(parent TreeParentId) *[]TreeId
	// Return the number of children of the target node.
	ChildrenNum(parent TreeParentId) *uint32
	// Return whether target node exists.
	Contains(target TreeId) bool
	// Create a new tree node and return the [`TreeID`].
	//
	// If the `parent` is `None`, the created node is the root of a tree.
	// Otherwise, the created node is a child of the parent tree node.
	Create(parent TreeParentId) (TreeId, error)
	// Create a new tree node at the given index and return the [`TreeID`].
	//
	// If the `parent` is `None`, the created node is the root of a tree.
	// If the `index` is greater than the number of children of the parent, error will be returned.
	CreateAt(parent TreeParentId, index uint32) (TreeId, error)
	// Delete a tree node.
	//
	// Note: If the deleted node has children, the children do not appear in the state
	// rather than actually being deleted.
	Delete(target TreeId) error
	// Disable the fractional index generation when you don't need the Tree's siblings to be sorted.
	// The fractional index will always be set to the same default value 0.
	//
	// After calling this, you cannot use `tree.moveTo()`, `tree.moveBefore()`, `tree.moveAfter()`,
	// and `tree.createAt()`.
	DisableFractionalIndex()
	// Get the LoroDoc from this container
	Doc() **LoroDoc
	// Enable fractional index for Tree Position.
	//
	// The jitter is used to avoid conflicts when multiple users are creating the node at the same position.
	// value 0 is default, which means no jitter, any value larger than 0 will enable jitter.
	//
	// Generally speaking, jitter will affect the growth rate of document size.
	// [Read more about it](https://www.loro.dev/blog/movable-tree#implementation-and-encoding-size)
	EnableFractionalIndex(jitter uint8)
	// Return the fractional index of the target node with hex format.
	FractionalIndex(target TreeId) *string
	// If a detached container is attached, this method will return its corresponding attached handler.
	GetAttached() **LoroTree
	// Get the last move id of the target node.
	GetLastMoveId(target TreeId) *Id
	// Get the associated metadata map handler of a tree node.
	GetMeta(target TreeId) (*LoroMap, error)
	// Return the flat array of the forest.
	//
	// Note: the metadata will be not resolved. So if you don't only care about hierarchy
	// but also the metadata, you should use `get_value_with_meta()`.
	GetValue() LoroValue
	// Return the flat array of the forest, each node is with metadata.
	GetValueWithMeta() LoroValue
	// Return container id of the tree.
	Id() ContainerId
	// Whether the container is attached to a document
	//
	// The edits on a detached container will not be persisted.
	// To attach the container to the document, please insert it into an attached container.
	IsAttached() bool
	// Whether the container is deleted.
	IsDeleted() bool
	// Whether the fractional index is enabled.
	IsFractionalIndexEnabled() bool
	// Return whether target node is deleted.
	//
	// # Errors
	// - If the target node does not exist, return `LoroTreeError::TreeNodeNotExist`.
	IsNodeDeleted(target TreeId) (bool, error)
	// Move the `target` node to be a child of the `parent` node.
	//
	// If the `parent` is `None`, the `target` node will be a root.
	Mov(target TreeId, parent TreeParentId) error
	// Move the `target` node to be a child after the `after` node with the same parent.
	MovAfter(target TreeId, after TreeId) error
	// Move the `target` node to be a child before the `before` node with the same parent.
	MovBefore(target TreeId, before TreeId) error
	// Move the `target` node to be a child of the `parent` node at the given index.
	// If the `parent` is `None`, the `target` node will be a root.
	MovTo(target TreeId, parent TreeParentId, to uint32) error
	// Return all nodes, including deleted nodes
	Nodes() []TreeId
	// Return the parent of target node.
	//
	// - If the target node does not exist, throws Error.
	// - If the target node is a root node, return nil.
	Parent(target TreeId) (TreeParentId, error)
	// Get the root nodes of the forest.
	Roots() []TreeId
}
type LoroTree struct {
	ffiObject FfiObject
}

// Create a new container that is detached from the document.
//
// The edits on a detached container will not be persisted.
// To attach the container to the document, please insert it into an attached container.
func NewLoroTree() *LoroTree {
	return FfiConverterLoroTreeINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_constructor_lorotree_new(_uniffiStatus)
	}))
}

// Return all children of the target node.
//
// If the parent node does not exist, return `None`.
func (_self *LoroTree) Children(parent TreeParentId) *[]TreeId {
	_pointer := _self.ffiObject.incrementPointer("*LoroTree")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalSequenceTreeIdINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorotree_children(
				_pointer, FfiConverterTreeParentIdINSTANCE.Lower(parent), _uniffiStatus),
		}
	}))
}

// Return the number of children of the target node.
func (_self *LoroTree) ChildrenNum(parent TreeParentId) *uint32 {
	_pointer := _self.ffiObject.incrementPointer("*LoroTree")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalUint32INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorotree_children_num(
				_pointer, FfiConverterTreeParentIdINSTANCE.Lower(parent), _uniffiStatus),
		}
	}))
}

// Return whether target node exists.
func (_self *LoroTree) Contains(target TreeId) bool {
	_pointer := _self.ffiObject.incrementPointer("*LoroTree")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_loro_fn_method_lorotree_contains(
			_pointer, FfiConverterTreeIdINSTANCE.Lower(target), _uniffiStatus)
	}))
}

// Create a new tree node and return the [`TreeID`].
//
// If the `parent` is `None`, the created node is the root of a tree.
// Otherwise, the created node is a child of the parent tree node.
func (_self *LoroTree) Create(parent TreeParentId) (TreeId, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroTree")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorotree_create(
				_pointer, FfiConverterTreeParentIdINSTANCE.Lower(parent), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue TreeId
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTreeIdINSTANCE.Lift(_uniffiRV), nil
	}
}

// Create a new tree node at the given index and return the [`TreeID`].
//
// If the `parent` is `None`, the created node is the root of a tree.
// If the `index` is greater than the number of children of the parent, error will be returned.
func (_self *LoroTree) CreateAt(parent TreeParentId, index uint32) (TreeId, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroTree")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorotree_create_at(
				_pointer, FfiConverterTreeParentIdINSTANCE.Lower(parent), FfiConverterUint32INSTANCE.Lower(index), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue TreeId
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTreeIdINSTANCE.Lift(_uniffiRV), nil
	}
}

// Delete a tree node.
//
// Note: If the deleted node has children, the children do not appear in the state
// rather than actually being deleted.
func (_self *LoroTree) Delete(target TreeId) error {
	_pointer := _self.ffiObject.incrementPointer("*LoroTree")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorotree_delete(
			_pointer, FfiConverterTreeIdINSTANCE.Lower(target), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Disable the fractional index generation when you don't need the Tree's siblings to be sorted.
// The fractional index will always be set to the same default value 0.
//
// After calling this, you cannot use `tree.moveTo()`, `tree.moveBefore()`, `tree.moveAfter()`,
// and `tree.createAt()`.
func (_self *LoroTree) DisableFractionalIndex() {
	_pointer := _self.ffiObject.incrementPointer("*LoroTree")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorotree_disable_fractional_index(
			_pointer, _uniffiStatus)
		return false
	})
}

// Get the LoroDoc from this container
func (_self *LoroTree) Doc() **LoroDoc {
	_pointer := _self.ffiObject.incrementPointer("*LoroTree")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalLoroDocINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorotree_doc(
				_pointer, _uniffiStatus),
		}
	}))
}

// Enable fractional index for Tree Position.
//
// The jitter is used to avoid conflicts when multiple users are creating the node at the same position.
// value 0 is default, which means no jitter, any value larger than 0 will enable jitter.
//
// Generally speaking, jitter will affect the growth rate of document size.
// [Read more about it](https://www.loro.dev/blog/movable-tree#implementation-and-encoding-size)
func (_self *LoroTree) EnableFractionalIndex(jitter uint8) {
	_pointer := _self.ffiObject.incrementPointer("*LoroTree")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorotree_enable_fractional_index(
			_pointer, FfiConverterUint8INSTANCE.Lower(jitter), _uniffiStatus)
		return false
	})
}

// Return the fractional index of the target node with hex format.
func (_self *LoroTree) FractionalIndex(target TreeId) *string {
	_pointer := _self.ffiObject.incrementPointer("*LoroTree")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorotree_fractional_index(
				_pointer, FfiConverterTreeIdINSTANCE.Lower(target), _uniffiStatus),
		}
	}))
}

// If a detached container is attached, this method will return its corresponding attached handler.
func (_self *LoroTree) GetAttached() **LoroTree {
	_pointer := _self.ffiObject.incrementPointer("*LoroTree")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalLoroTreeINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorotree_get_attached(
				_pointer, _uniffiStatus),
		}
	}))
}

// Get the last move id of the target node.
func (_self *LoroTree) GetLastMoveId(target TreeId) *Id {
	_pointer := _self.ffiObject.incrementPointer("*LoroTree")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalIdINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorotree_get_last_move_id(
				_pointer, FfiConverterTreeIdINSTANCE.Lower(target), _uniffiStatus),
		}
	}))
}

// Get the associated metadata map handler of a tree node.
func (_self *LoroTree) GetMeta(target TreeId) (*LoroMap, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroTree")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_method_lorotree_get_meta(
			_pointer, FfiConverterTreeIdINSTANCE.Lower(target), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LoroMap
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLoroMapINSTANCE.Lift(_uniffiRV), nil
	}
}

// Return the flat array of the forest.
//
// Note: the metadata will be not resolved. So if you don't only care about hierarchy
// but also the metadata, you should use `get_value_with_meta()`.
func (_self *LoroTree) GetValue() LoroValue {
	_pointer := _self.ffiObject.incrementPointer("*LoroTree")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterLoroValueINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorotree_get_value(
				_pointer, _uniffiStatus),
		}
	}))
}

// Return the flat array of the forest, each node is with metadata.
func (_self *LoroTree) GetValueWithMeta() LoroValue {
	_pointer := _self.ffiObject.incrementPointer("*LoroTree")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterLoroValueINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorotree_get_value_with_meta(
				_pointer, _uniffiStatus),
		}
	}))
}

// Return container id of the tree.
func (_self *LoroTree) Id() ContainerId {
	_pointer := _self.ffiObject.incrementPointer("*LoroTree")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterContainerIdINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorotree_id(
				_pointer, _uniffiStatus),
		}
	}))
}

// Whether the container is attached to a document
//
// The edits on a detached container will not be persisted.
// To attach the container to the document, please insert it into an attached container.
func (_self *LoroTree) IsAttached() bool {
	_pointer := _self.ffiObject.incrementPointer("*LoroTree")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_loro_fn_method_lorotree_is_attached(
			_pointer, _uniffiStatus)
	}))
}

// Whether the container is deleted.
func (_self *LoroTree) IsDeleted() bool {
	_pointer := _self.ffiObject.incrementPointer("*LoroTree")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_loro_fn_method_lorotree_is_deleted(
			_pointer, _uniffiStatus)
	}))
}

// Whether the fractional index is enabled.
func (_self *LoroTree) IsFractionalIndexEnabled() bool {
	_pointer := _self.ffiObject.incrementPointer("*LoroTree")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_loro_fn_method_lorotree_is_fractional_index_enabled(
			_pointer, _uniffiStatus)
	}))
}

// Return whether target node is deleted.
//
// # Errors
// - If the target node does not exist, return `LoroTreeError::TreeNodeNotExist`.
func (_self *LoroTree) IsNodeDeleted(target TreeId) (bool, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroTree")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_loro_fn_method_lorotree_is_node_deleted(
			_pointer, FfiConverterTreeIdINSTANCE.Lower(target), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue bool
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBoolINSTANCE.Lift(_uniffiRV), nil
	}
}

// Move the `target` node to be a child of the `parent` node.
//
// If the `parent` is `None`, the `target` node will be a root.
func (_self *LoroTree) Mov(target TreeId, parent TreeParentId) error {
	_pointer := _self.ffiObject.incrementPointer("*LoroTree")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorotree_mov(
			_pointer, FfiConverterTreeIdINSTANCE.Lower(target), FfiConverterTreeParentIdINSTANCE.Lower(parent), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Move the `target` node to be a child after the `after` node with the same parent.
func (_self *LoroTree) MovAfter(target TreeId, after TreeId) error {
	_pointer := _self.ffiObject.incrementPointer("*LoroTree")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorotree_mov_after(
			_pointer, FfiConverterTreeIdINSTANCE.Lower(target), FfiConverterTreeIdINSTANCE.Lower(after), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Move the `target` node to be a child before the `before` node with the same parent.
func (_self *LoroTree) MovBefore(target TreeId, before TreeId) error {
	_pointer := _self.ffiObject.incrementPointer("*LoroTree")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorotree_mov_before(
			_pointer, FfiConverterTreeIdINSTANCE.Lower(target), FfiConverterTreeIdINSTANCE.Lower(before), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Move the `target` node to be a child of the `parent` node at the given index.
// If the `parent` is `None`, the `target` node will be a root.
func (_self *LoroTree) MovTo(target TreeId, parent TreeParentId, to uint32) error {
	_pointer := _self.ffiObject.incrementPointer("*LoroTree")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_lorotree_mov_to(
			_pointer, FfiConverterTreeIdINSTANCE.Lower(target), FfiConverterTreeParentIdINSTANCE.Lower(parent), FfiConverterUint32INSTANCE.Lower(to), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Return all nodes, including deleted nodes
func (_self *LoroTree) Nodes() []TreeId {
	_pointer := _self.ffiObject.incrementPointer("*LoroTree")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceTreeIdINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorotree_nodes(
				_pointer, _uniffiStatus),
		}
	}))
}

// Return the parent of target node.
//
// - If the target node does not exist, throws Error.
// - If the target node is a root node, return nil.
func (_self *LoroTree) Parent(target TreeId) (TreeParentId, error) {
	_pointer := _self.ffiObject.incrementPointer("*LoroTree")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorotree_parent(
				_pointer, FfiConverterTreeIdINSTANCE.Lower(target), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue TreeParentId
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTreeParentIdINSTANCE.Lift(_uniffiRV), nil
	}
}

// Get the root nodes of the forest.
func (_self *LoroTree) Roots() []TreeId {
	_pointer := _self.ffiObject.incrementPointer("*LoroTree")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceTreeIdINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorotree_roots(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *LoroTree) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterLoroTree struct{}

var FfiConverterLoroTreeINSTANCE = FfiConverterLoroTree{}

func (c FfiConverterLoroTree) Lift(pointer unsafe.Pointer) *LoroTree {
	result := &LoroTree{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_loro_fn_clone_lorotree(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_loro_fn_free_lorotree(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*LoroTree).Destroy)
	return result
}

func (c FfiConverterLoroTree) Read(reader io.Reader) *LoroTree {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterLoroTree) Lower(value *LoroTree) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*LoroTree")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterLoroTree) Write(writer io.Writer, value *LoroTree) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerLoroTree struct{}

func (_ FfiDestroyerLoroTree) Destroy(value *LoroTree) {
	value.Destroy()
}

type LoroUnknownInterface interface {
	// Get the container id.
	Id() ContainerId
}
type LoroUnknown struct {
	ffiObject FfiObject
}

// Get the container id.
func (_self *LoroUnknown) Id() ContainerId {
	_pointer := _self.ffiObject.incrementPointer("*LoroUnknown")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterContainerIdINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorounknown_id(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *LoroUnknown) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterLoroUnknown struct{}

var FfiConverterLoroUnknownINSTANCE = FfiConverterLoroUnknown{}

func (c FfiConverterLoroUnknown) Lift(pointer unsafe.Pointer) *LoroUnknown {
	result := &LoroUnknown{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_loro_fn_clone_lorounknown(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_loro_fn_free_lorounknown(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*LoroUnknown).Destroy)
	return result
}

func (c FfiConverterLoroUnknown) Read(reader io.Reader) *LoroUnknown {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterLoroUnknown) Lower(value *LoroUnknown) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*LoroUnknown")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterLoroUnknown) Write(writer io.Writer, value *LoroUnknown) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerLoroUnknown struct{}

func (_ FfiDestroyerLoroUnknown) Destroy(value *LoroUnknown) {
	value.Destroy()
}

type LoroValueLike interface {
	AsLoroValue() LoroValue
}
type LoroValueLikeImpl struct {
	ffiObject FfiObject
}

func (_self *LoroValueLikeImpl) AsLoroValue() LoroValue {
	_pointer := _self.ffiObject.incrementPointer("LoroValueLike")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterLoroValueINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_lorovaluelike_as_loro_value(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *LoroValueLikeImpl) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterLoroValueLike struct {
	handleMap *concurrentHandleMap[LoroValueLike]
}

var FfiConverterLoroValueLikeINSTANCE = FfiConverterLoroValueLike{
	handleMap: newConcurrentHandleMap[LoroValueLike](),
}

func (c FfiConverterLoroValueLike) Lift(pointer unsafe.Pointer) LoroValueLike {
	result := &LoroValueLikeImpl{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_loro_fn_clone_lorovaluelike(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_loro_fn_free_lorovaluelike(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*LoroValueLikeImpl).Destroy)
	return result
}

func (c FfiConverterLoroValueLike) Read(reader io.Reader) LoroValueLike {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterLoroValueLike) Lower(value LoroValueLike) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := unsafe.Pointer(uintptr(c.handleMap.insert(value)))
	return pointer

}

func (c FfiConverterLoroValueLike) Write(writer io.Writer, value LoroValueLike) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerLoroValueLike struct{}

func (_ FfiDestroyerLoroValueLike) Destroy(value LoroValueLike) {
	if val, ok := value.(*LoroValueLikeImpl); ok {
		val.Destroy()
	} else {
		panic("Expected *LoroValueLikeImpl")
	}
}

//export loro_cgo_dispatchCallbackInterfaceLoroValueLikeMethod0
func loro_cgo_dispatchCallbackInterfaceLoroValueLikeMethod0(uniffiHandle C.uint64_t, uniffiOutReturn *C.RustBuffer, callStatus *C.RustCallStatus) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterLoroValueLikeINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	res :=
		uniffiObj.AsLoroValue()

	*uniffiOutReturn = FfiConverterLoroValueINSTANCE.Lower(res)
}

var UniffiVTableCallbackInterfaceLoroValueLikeINSTANCE = C.UniffiVTableCallbackInterfaceLoroValueLike{
	asLoroValue: (C.UniffiCallbackInterfaceLoroValueLikeMethod0)(C.loro_cgo_dispatchCallbackInterfaceLoroValueLikeMethod0),

	uniffiFree: (C.UniffiCallbackInterfaceFree)(C.loro_cgo_dispatchCallbackInterfaceLoroValueLikeFree),
}

//export loro_cgo_dispatchCallbackInterfaceLoroValueLikeFree
func loro_cgo_dispatchCallbackInterfaceLoroValueLikeFree(handle C.uint64_t) {
	FfiConverterLoroValueLikeINSTANCE.handleMap.remove(uint64(handle))
}

func (c FfiConverterLoroValueLike) register() {
	C.uniffi_loro_fn_init_callback_vtable_lorovaluelike(&UniffiVTableCallbackInterfaceLoroValueLikeINSTANCE)
}

type OnPop interface {
	OnPop(undoOrRedo UndoOrRedo, span CounterSpan, undoMeta UndoItemMeta)
}
type OnPopImpl struct {
	ffiObject FfiObject
}

func (_self *OnPopImpl) OnPop(undoOrRedo UndoOrRedo, span CounterSpan, undoMeta UndoItemMeta) {
	_pointer := _self.ffiObject.incrementPointer("OnPop")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_onpop_on_pop(
			_pointer, FfiConverterUndoOrRedoINSTANCE.Lower(undoOrRedo), FfiConverterCounterSpanINSTANCE.Lower(span), FfiConverterUndoItemMetaINSTANCE.Lower(undoMeta), _uniffiStatus)
		return false
	})
}
func (object *OnPopImpl) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterOnPop struct {
	handleMap *concurrentHandleMap[OnPop]
}

var FfiConverterOnPopINSTANCE = FfiConverterOnPop{
	handleMap: newConcurrentHandleMap[OnPop](),
}

func (c FfiConverterOnPop) Lift(pointer unsafe.Pointer) OnPop {
	result := &OnPopImpl{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_loro_fn_clone_onpop(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_loro_fn_free_onpop(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*OnPopImpl).Destroy)
	return result
}

func (c FfiConverterOnPop) Read(reader io.Reader) OnPop {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterOnPop) Lower(value OnPop) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := unsafe.Pointer(uintptr(c.handleMap.insert(value)))
	return pointer

}

func (c FfiConverterOnPop) Write(writer io.Writer, value OnPop) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerOnPop struct{}

func (_ FfiDestroyerOnPop) Destroy(value OnPop) {
	if val, ok := value.(*OnPopImpl); ok {
		val.Destroy()
	} else {
		panic("Expected *OnPopImpl")
	}
}

//export loro_cgo_dispatchCallbackInterfaceOnPopMethod0
func loro_cgo_dispatchCallbackInterfaceOnPopMethod0(uniffiHandle C.uint64_t, undoOrRedo C.RustBuffer, span C.RustBuffer, undoMeta C.RustBuffer, uniffiOutReturn *C.void, callStatus *C.RustCallStatus) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterOnPopINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	uniffiObj.OnPop(
		FfiConverterUndoOrRedoINSTANCE.Lift(GoRustBuffer{
			inner: undoOrRedo,
		}),
		FfiConverterCounterSpanINSTANCE.Lift(GoRustBuffer{
			inner: span,
		}),
		FfiConverterUndoItemMetaINSTANCE.Lift(GoRustBuffer{
			inner: undoMeta,
		}),
	)

}

var UniffiVTableCallbackInterfaceOnPopINSTANCE = C.UniffiVTableCallbackInterfaceOnPop{
	onPop: (C.UniffiCallbackInterfaceOnPopMethod0)(C.loro_cgo_dispatchCallbackInterfaceOnPopMethod0),

	uniffiFree: (C.UniffiCallbackInterfaceFree)(C.loro_cgo_dispatchCallbackInterfaceOnPopFree),
}

//export loro_cgo_dispatchCallbackInterfaceOnPopFree
func loro_cgo_dispatchCallbackInterfaceOnPopFree(handle C.uint64_t) {
	FfiConverterOnPopINSTANCE.handleMap.remove(uint64(handle))
}

func (c FfiConverterOnPop) register() {
	C.uniffi_loro_fn_init_callback_vtable_onpop(&UniffiVTableCallbackInterfaceOnPopINSTANCE)
}

type OnPush interface {
	OnPush(undoOrRedo UndoOrRedo, span CounterSpan, diffEvent *DiffEvent) UndoItemMeta
}
type OnPushImpl struct {
	ffiObject FfiObject
}

func (_self *OnPushImpl) OnPush(undoOrRedo UndoOrRedo, span CounterSpan, diffEvent *DiffEvent) UndoItemMeta {
	_pointer := _self.ffiObject.incrementPointer("OnPush")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUndoItemMetaINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_onpush_on_push(
				_pointer, FfiConverterUndoOrRedoINSTANCE.Lower(undoOrRedo), FfiConverterCounterSpanINSTANCE.Lower(span), FfiConverterOptionalDiffEventINSTANCE.Lower(diffEvent), _uniffiStatus),
		}
	}))
}
func (object *OnPushImpl) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterOnPush struct {
	handleMap *concurrentHandleMap[OnPush]
}

var FfiConverterOnPushINSTANCE = FfiConverterOnPush{
	handleMap: newConcurrentHandleMap[OnPush](),
}

func (c FfiConverterOnPush) Lift(pointer unsafe.Pointer) OnPush {
	result := &OnPushImpl{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_loro_fn_clone_onpush(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_loro_fn_free_onpush(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*OnPushImpl).Destroy)
	return result
}

func (c FfiConverterOnPush) Read(reader io.Reader) OnPush {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterOnPush) Lower(value OnPush) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := unsafe.Pointer(uintptr(c.handleMap.insert(value)))
	return pointer

}

func (c FfiConverterOnPush) Write(writer io.Writer, value OnPush) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerOnPush struct{}

func (_ FfiDestroyerOnPush) Destroy(value OnPush) {
	if val, ok := value.(*OnPushImpl); ok {
		val.Destroy()
	} else {
		panic("Expected *OnPushImpl")
	}
}

//export loro_cgo_dispatchCallbackInterfaceOnPushMethod0
func loro_cgo_dispatchCallbackInterfaceOnPushMethod0(uniffiHandle C.uint64_t, undoOrRedo C.RustBuffer, span C.RustBuffer, diffEvent C.RustBuffer, uniffiOutReturn *C.RustBuffer, callStatus *C.RustCallStatus) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterOnPushINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	res :=
		uniffiObj.OnPush(
			FfiConverterUndoOrRedoINSTANCE.Lift(GoRustBuffer{
				inner: undoOrRedo,
			}),
			FfiConverterCounterSpanINSTANCE.Lift(GoRustBuffer{
				inner: span,
			}),
			FfiConverterOptionalDiffEventINSTANCE.Lift(GoRustBuffer{
				inner: diffEvent,
			}),
		)

	*uniffiOutReturn = FfiConverterUndoItemMetaINSTANCE.Lower(res)
}

var UniffiVTableCallbackInterfaceOnPushINSTANCE = C.UniffiVTableCallbackInterfaceOnPush{
	onPush: (C.UniffiCallbackInterfaceOnPushMethod0)(C.loro_cgo_dispatchCallbackInterfaceOnPushMethod0),

	uniffiFree: (C.UniffiCallbackInterfaceFree)(C.loro_cgo_dispatchCallbackInterfaceOnPushFree),
}

//export loro_cgo_dispatchCallbackInterfaceOnPushFree
func loro_cgo_dispatchCallbackInterfaceOnPushFree(handle C.uint64_t) {
	FfiConverterOnPushINSTANCE.handleMap.remove(uint64(handle))
}

func (c FfiConverterOnPush) register() {
	C.uniffi_loro_fn_init_callback_vtable_onpush(&UniffiVTableCallbackInterfaceOnPushINSTANCE)
}

type PreCommitCallback interface {
	OnPreCommit(payload PreCommitCallbackPayload)
}
type PreCommitCallbackImpl struct {
	ffiObject FfiObject
}

func (_self *PreCommitCallbackImpl) OnPreCommit(payload PreCommitCallbackPayload) {
	_pointer := _self.ffiObject.incrementPointer("PreCommitCallback")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_precommitcallback_on_pre_commit(
			_pointer, FfiConverterPreCommitCallbackPayloadINSTANCE.Lower(payload), _uniffiStatus)
		return false
	})
}
func (object *PreCommitCallbackImpl) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterPreCommitCallback struct {
	handleMap *concurrentHandleMap[PreCommitCallback]
}

var FfiConverterPreCommitCallbackINSTANCE = FfiConverterPreCommitCallback{
	handleMap: newConcurrentHandleMap[PreCommitCallback](),
}

func (c FfiConverterPreCommitCallback) Lift(pointer unsafe.Pointer) PreCommitCallback {
	result := &PreCommitCallbackImpl{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_loro_fn_clone_precommitcallback(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_loro_fn_free_precommitcallback(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*PreCommitCallbackImpl).Destroy)
	return result
}

func (c FfiConverterPreCommitCallback) Read(reader io.Reader) PreCommitCallback {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterPreCommitCallback) Lower(value PreCommitCallback) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := unsafe.Pointer(uintptr(c.handleMap.insert(value)))
	return pointer

}

func (c FfiConverterPreCommitCallback) Write(writer io.Writer, value PreCommitCallback) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerPreCommitCallback struct{}

func (_ FfiDestroyerPreCommitCallback) Destroy(value PreCommitCallback) {
	if val, ok := value.(*PreCommitCallbackImpl); ok {
		val.Destroy()
	} else {
		panic("Expected *PreCommitCallbackImpl")
	}
}

//export loro_cgo_dispatchCallbackInterfacePreCommitCallbackMethod0
func loro_cgo_dispatchCallbackInterfacePreCommitCallbackMethod0(uniffiHandle C.uint64_t, payload C.RustBuffer, uniffiOutReturn *C.void, callStatus *C.RustCallStatus) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterPreCommitCallbackINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	uniffiObj.OnPreCommit(
		FfiConverterPreCommitCallbackPayloadINSTANCE.Lift(GoRustBuffer{
			inner: payload,
		}),
	)

}

var UniffiVTableCallbackInterfacePreCommitCallbackINSTANCE = C.UniffiVTableCallbackInterfacePreCommitCallback{
	onPreCommit: (C.UniffiCallbackInterfacePreCommitCallbackMethod0)(C.loro_cgo_dispatchCallbackInterfacePreCommitCallbackMethod0),

	uniffiFree: (C.UniffiCallbackInterfaceFree)(C.loro_cgo_dispatchCallbackInterfacePreCommitCallbackFree),
}

//export loro_cgo_dispatchCallbackInterfacePreCommitCallbackFree
func loro_cgo_dispatchCallbackInterfacePreCommitCallbackFree(handle C.uint64_t) {
	FfiConverterPreCommitCallbackINSTANCE.handleMap.remove(uint64(handle))
}

func (c FfiConverterPreCommitCallback) register() {
	C.uniffi_loro_fn_init_callback_vtable_precommitcallback(&UniffiVTableCallbackInterfacePreCommitCallbackINSTANCE)
}

type StyleConfigMapInterface interface {
	Get(key string) *StyleConfig
	Insert(key string, value StyleConfig)
}
type StyleConfigMap struct {
	ffiObject FfiObject
}

func NewStyleConfigMap() *StyleConfigMap {
	return FfiConverterStyleConfigMapINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_constructor_styleconfigmap_new(_uniffiStatus)
	}))
}

func StyleConfigMapDefaultRichTextConfig() *StyleConfigMap {
	return FfiConverterStyleConfigMapINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_constructor_styleconfigmap_default_rich_text_config(_uniffiStatus)
	}))
}

func (_self *StyleConfigMap) Get(key string) *StyleConfig {
	_pointer := _self.ffiObject.incrementPointer("*StyleConfigMap")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalStyleConfigINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_styleconfigmap_get(
				_pointer, FfiConverterStringINSTANCE.Lower(key), _uniffiStatus),
		}
	}))
}

func (_self *StyleConfigMap) Insert(key string, value StyleConfig) {
	_pointer := _self.ffiObject.incrementPointer("*StyleConfigMap")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_styleconfigmap_insert(
			_pointer, FfiConverterStringINSTANCE.Lower(key), FfiConverterStyleConfigINSTANCE.Lower(value), _uniffiStatus)
		return false
	})
}
func (object *StyleConfigMap) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterStyleConfigMap struct{}

var FfiConverterStyleConfigMapINSTANCE = FfiConverterStyleConfigMap{}

func (c FfiConverterStyleConfigMap) Lift(pointer unsafe.Pointer) *StyleConfigMap {
	result := &StyleConfigMap{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_loro_fn_clone_styleconfigmap(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_loro_fn_free_styleconfigmap(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*StyleConfigMap).Destroy)
	return result
}

func (c FfiConverterStyleConfigMap) Read(reader io.Reader) *StyleConfigMap {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterStyleConfigMap) Lower(value *StyleConfigMap) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*StyleConfigMap")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterStyleConfigMap) Write(writer io.Writer, value *StyleConfigMap) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerStyleConfigMap struct{}

func (_ FfiDestroyerStyleConfigMap) Destroy(value *StyleConfigMap) {
	value.Destroy()
}

type Subscriber interface {
	OnDiff(diff DiffEvent)
}
type SubscriberImpl struct {
	ffiObject FfiObject
}

func (_self *SubscriberImpl) OnDiff(diff DiffEvent) {
	_pointer := _self.ffiObject.incrementPointer("Subscriber")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_subscriber_on_diff(
			_pointer, FfiConverterDiffEventINSTANCE.Lower(diff), _uniffiStatus)
		return false
	})
}
func (object *SubscriberImpl) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterSubscriber struct {
	handleMap *concurrentHandleMap[Subscriber]
}

var FfiConverterSubscriberINSTANCE = FfiConverterSubscriber{
	handleMap: newConcurrentHandleMap[Subscriber](),
}

func (c FfiConverterSubscriber) Lift(pointer unsafe.Pointer) Subscriber {
	result := &SubscriberImpl{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_loro_fn_clone_subscriber(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_loro_fn_free_subscriber(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*SubscriberImpl).Destroy)
	return result
}

func (c FfiConverterSubscriber) Read(reader io.Reader) Subscriber {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterSubscriber) Lower(value Subscriber) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := unsafe.Pointer(uintptr(c.handleMap.insert(value)))
	return pointer

}

func (c FfiConverterSubscriber) Write(writer io.Writer, value Subscriber) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerSubscriber struct{}

func (_ FfiDestroyerSubscriber) Destroy(value Subscriber) {
	if val, ok := value.(*SubscriberImpl); ok {
		val.Destroy()
	} else {
		panic("Expected *SubscriberImpl")
	}
}

//export loro_cgo_dispatchCallbackInterfaceSubscriberMethod0
func loro_cgo_dispatchCallbackInterfaceSubscriberMethod0(uniffiHandle C.uint64_t, diff C.RustBuffer, uniffiOutReturn *C.void, callStatus *C.RustCallStatus) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterSubscriberINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	uniffiObj.OnDiff(
		FfiConverterDiffEventINSTANCE.Lift(GoRustBuffer{
			inner: diff,
		}),
	)

}

var UniffiVTableCallbackInterfaceSubscriberINSTANCE = C.UniffiVTableCallbackInterfaceSubscriber{
	onDiff: (C.UniffiCallbackInterfaceSubscriberMethod0)(C.loro_cgo_dispatchCallbackInterfaceSubscriberMethod0),

	uniffiFree: (C.UniffiCallbackInterfaceFree)(C.loro_cgo_dispatchCallbackInterfaceSubscriberFree),
}

//export loro_cgo_dispatchCallbackInterfaceSubscriberFree
func loro_cgo_dispatchCallbackInterfaceSubscriberFree(handle C.uint64_t) {
	FfiConverterSubscriberINSTANCE.handleMap.remove(uint64(handle))
}

func (c FfiConverterSubscriber) register() {
	C.uniffi_loro_fn_init_callback_vtable_subscriber(&UniffiVTableCallbackInterfaceSubscriberINSTANCE)
}

// A handle to a subscription created by GPUI. When dropped, the subscription
// is cancelled and the callback will no longer be invoked.
type SubscriptionInterface interface {
	// Detaches the subscription from this handle. The callback will
	// continue to be invoked until the views or models it has been
	// subscribed to are dropped
	Detach()
	// Unsubscribes the subscription.
	Unsubscribe()
}

// A handle to a subscription created by GPUI. When dropped, the subscription
// is cancelled and the callback will no longer be invoked.
type Subscription struct {
	ffiObject FfiObject
}

// Detaches the subscription from this handle. The callback will
// continue to be invoked until the views or models it has been
// subscribed to are dropped
func (_self *Subscription) Detach() {
	_pointer := _self.ffiObject.incrementPointer("*Subscription")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_subscription_detach(
			_pointer, _uniffiStatus)
		return false
	})
}

// Unsubscribes the subscription.
func (_self *Subscription) Unsubscribe() {
	_pointer := _self.ffiObject.incrementPointer("*Subscription")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_subscription_unsubscribe(
			_pointer, _uniffiStatus)
		return false
	})
}
func (object *Subscription) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterSubscription struct{}

var FfiConverterSubscriptionINSTANCE = FfiConverterSubscription{}

func (c FfiConverterSubscription) Lift(pointer unsafe.Pointer) *Subscription {
	result := &Subscription{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_loro_fn_clone_subscription(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_loro_fn_free_subscription(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Subscription).Destroy)
	return result
}

func (c FfiConverterSubscription) Read(reader io.Reader) *Subscription {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterSubscription) Lower(value *Subscription) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Subscription")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterSubscription) Write(writer io.Writer, value *Subscription) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerSubscription struct{}

func (_ FfiDestroyerSubscription) Destroy(value *Subscription) {
	value.Destroy()
}

type UndoManagerInterface interface {
	// If a local event's origin matches the given prefix, it will not be recorded in the
	// undo stack.
	AddExcludeOriginPrefix(prefix string)
	// Whether the undo manager can redo.
	CanRedo() bool
	// Whether the undo manager can undo.
	CanUndo() bool
	// Record a new checkpoint.
	RecordNewCheckpoint() error
	// Redo the last change made by the peer.
	Redo() (bool, error)
	// Set the maximum number of undo steps. The default value is 100.
	SetMaxUndoSteps(size uint32)
	// Set the merge interval in ms. The default value is 0, which means no merge.
	SetMergeInterval(interval int64)
	// Set the listener for pop events.
	// The listener will be called when an undo/redo item is popped from the stack.
	SetOnPop(onPop *OnPop)
	// Set the listener for push events.
	// The listener will be called when a new undo/redo item is pushed into the stack.
	SetOnPush(onPush *OnPush)
	// Undo the last change made by the peer.
	Undo() (bool, error)
}
type UndoManager struct {
	ffiObject FfiObject
}

// Create a new UndoManager.
func NewUndoManager(doc *LoroDoc) *UndoManager {
	return FfiConverterUndoManagerINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_constructor_undomanager_new(FfiConverterLoroDocINSTANCE.Lower(doc), _uniffiStatus)
	}))
}

// If a local event's origin matches the given prefix, it will not be recorded in the
// undo stack.
func (_self *UndoManager) AddExcludeOriginPrefix(prefix string) {
	_pointer := _self.ffiObject.incrementPointer("*UndoManager")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_undomanager_add_exclude_origin_prefix(
			_pointer, FfiConverterStringINSTANCE.Lower(prefix), _uniffiStatus)
		return false
	})
}

// Whether the undo manager can redo.
func (_self *UndoManager) CanRedo() bool {
	_pointer := _self.ffiObject.incrementPointer("*UndoManager")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_loro_fn_method_undomanager_can_redo(
			_pointer, _uniffiStatus)
	}))
}

// Whether the undo manager can undo.
func (_self *UndoManager) CanUndo() bool {
	_pointer := _self.ffiObject.incrementPointer("*UndoManager")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_loro_fn_method_undomanager_can_undo(
			_pointer, _uniffiStatus)
	}))
}

// Record a new checkpoint.
func (_self *UndoManager) RecordNewCheckpoint() error {
	_pointer := _self.ffiObject.incrementPointer("*UndoManager")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_undomanager_record_new_checkpoint(
			_pointer, _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Redo the last change made by the peer.
func (_self *UndoManager) Redo() (bool, error) {
	_pointer := _self.ffiObject.incrementPointer("*UndoManager")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_loro_fn_method_undomanager_redo(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue bool
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBoolINSTANCE.Lift(_uniffiRV), nil
	}
}

// Set the maximum number of undo steps. The default value is 100.
func (_self *UndoManager) SetMaxUndoSteps(size uint32) {
	_pointer := _self.ffiObject.incrementPointer("*UndoManager")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_undomanager_set_max_undo_steps(
			_pointer, FfiConverterUint32INSTANCE.Lower(size), _uniffiStatus)
		return false
	})
}

// Set the merge interval in ms. The default value is 0, which means no merge.
func (_self *UndoManager) SetMergeInterval(interval int64) {
	_pointer := _self.ffiObject.incrementPointer("*UndoManager")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_undomanager_set_merge_interval(
			_pointer, FfiConverterInt64INSTANCE.Lower(interval), _uniffiStatus)
		return false
	})
}

// Set the listener for pop events.
// The listener will be called when an undo/redo item is popped from the stack.
func (_self *UndoManager) SetOnPop(onPop *OnPop) {
	_pointer := _self.ffiObject.incrementPointer("*UndoManager")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_undomanager_set_on_pop(
			_pointer, FfiConverterOptionalOnPopINSTANCE.Lower(onPop), _uniffiStatus)
		return false
	})
}

// Set the listener for push events.
// The listener will be called when a new undo/redo item is pushed into the stack.
func (_self *UndoManager) SetOnPush(onPush *OnPush) {
	_pointer := _self.ffiObject.incrementPointer("*UndoManager")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_undomanager_set_on_push(
			_pointer, FfiConverterOptionalOnPushINSTANCE.Lower(onPush), _uniffiStatus)
		return false
	})
}

// Undo the last change made by the peer.
func (_self *UndoManager) Undo() (bool, error) {
	_pointer := _self.ffiObject.incrementPointer("*UndoManager")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_loro_fn_method_undomanager_undo(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue bool
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBoolINSTANCE.Lift(_uniffiRV), nil
	}
}
func (object *UndoManager) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterUndoManager struct{}

var FfiConverterUndoManagerINSTANCE = FfiConverterUndoManager{}

func (c FfiConverterUndoManager) Lift(pointer unsafe.Pointer) *UndoManager {
	result := &UndoManager{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_loro_fn_clone_undomanager(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_loro_fn_free_undomanager(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*UndoManager).Destroy)
	return result
}

func (c FfiConverterUndoManager) Read(reader io.Reader) *UndoManager {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterUndoManager) Lower(value *UndoManager) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*UndoManager")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterUndoManager) Write(writer io.Writer, value *UndoManager) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerUndoManager struct{}

func (_ FfiDestroyerUndoManager) Destroy(value *UndoManager) {
	value.Destroy()
}

type Unsubscriber interface {
	OnUnsubscribe()
}
type UnsubscriberImpl struct {
	ffiObject FfiObject
}

func (_self *UnsubscriberImpl) OnUnsubscribe() {
	_pointer := _self.ffiObject.incrementPointer("Unsubscriber")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_unsubscriber_on_unsubscribe(
			_pointer, _uniffiStatus)
		return false
	})
}
func (object *UnsubscriberImpl) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterUnsubscriber struct {
	handleMap *concurrentHandleMap[Unsubscriber]
}

var FfiConverterUnsubscriberINSTANCE = FfiConverterUnsubscriber{
	handleMap: newConcurrentHandleMap[Unsubscriber](),
}

func (c FfiConverterUnsubscriber) Lift(pointer unsafe.Pointer) Unsubscriber {
	result := &UnsubscriberImpl{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_loro_fn_clone_unsubscriber(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_loro_fn_free_unsubscriber(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*UnsubscriberImpl).Destroy)
	return result
}

func (c FfiConverterUnsubscriber) Read(reader io.Reader) Unsubscriber {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterUnsubscriber) Lower(value Unsubscriber) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := unsafe.Pointer(uintptr(c.handleMap.insert(value)))
	return pointer

}

func (c FfiConverterUnsubscriber) Write(writer io.Writer, value Unsubscriber) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerUnsubscriber struct{}

func (_ FfiDestroyerUnsubscriber) Destroy(value Unsubscriber) {
	if val, ok := value.(*UnsubscriberImpl); ok {
		val.Destroy()
	} else {
		panic("Expected *UnsubscriberImpl")
	}
}

//export loro_cgo_dispatchCallbackInterfaceUnsubscriberMethod0
func loro_cgo_dispatchCallbackInterfaceUnsubscriberMethod0(uniffiHandle C.uint64_t, uniffiOutReturn *C.void, callStatus *C.RustCallStatus) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterUnsubscriberINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	uniffiObj.OnUnsubscribe()

}

var UniffiVTableCallbackInterfaceUnsubscriberINSTANCE = C.UniffiVTableCallbackInterfaceUnsubscriber{
	onUnsubscribe: (C.UniffiCallbackInterfaceUnsubscriberMethod0)(C.loro_cgo_dispatchCallbackInterfaceUnsubscriberMethod0),

	uniffiFree: (C.UniffiCallbackInterfaceFree)(C.loro_cgo_dispatchCallbackInterfaceUnsubscriberFree),
}

//export loro_cgo_dispatchCallbackInterfaceUnsubscriberFree
func loro_cgo_dispatchCallbackInterfaceUnsubscriberFree(handle C.uint64_t) {
	FfiConverterUnsubscriberINSTANCE.handleMap.remove(uint64(handle))
}

func (c FfiConverterUnsubscriber) register() {
	C.uniffi_loro_fn_init_callback_vtable_unsubscriber(&UniffiVTableCallbackInterfaceUnsubscriberINSTANCE)
}

type ValueOrContainerInterface interface {
	AsContainer() *ContainerId
	AsLoroCounter() **LoroCounter
	AsLoroList() **LoroList
	AsLoroMap() **LoroMap
	AsLoroMovableList() **LoroMovableList
	AsLoroText() **LoroText
	AsLoroTree() **LoroTree
	AsLoroUnknown() **LoroUnknown
	AsValue() *LoroValue
	ContainerType() *ContainerType
	IsContainer() bool
	IsValue() bool
}
type ValueOrContainer struct {
	ffiObject FfiObject
}

func (_self *ValueOrContainer) AsContainer() *ContainerId {
	_pointer := _self.ffiObject.incrementPointer("*ValueOrContainer")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalContainerIdINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_valueorcontainer_as_container(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *ValueOrContainer) AsLoroCounter() **LoroCounter {
	_pointer := _self.ffiObject.incrementPointer("*ValueOrContainer")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalLoroCounterINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_valueorcontainer_as_loro_counter(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *ValueOrContainer) AsLoroList() **LoroList {
	_pointer := _self.ffiObject.incrementPointer("*ValueOrContainer")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalLoroListINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_valueorcontainer_as_loro_list(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *ValueOrContainer) AsLoroMap() **LoroMap {
	_pointer := _self.ffiObject.incrementPointer("*ValueOrContainer")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalLoroMapINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_valueorcontainer_as_loro_map(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *ValueOrContainer) AsLoroMovableList() **LoroMovableList {
	_pointer := _self.ffiObject.incrementPointer("*ValueOrContainer")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalLoroMovableListINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_valueorcontainer_as_loro_movable_list(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *ValueOrContainer) AsLoroText() **LoroText {
	_pointer := _self.ffiObject.incrementPointer("*ValueOrContainer")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalLoroTextINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_valueorcontainer_as_loro_text(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *ValueOrContainer) AsLoroTree() **LoroTree {
	_pointer := _self.ffiObject.incrementPointer("*ValueOrContainer")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalLoroTreeINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_valueorcontainer_as_loro_tree(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *ValueOrContainer) AsLoroUnknown() **LoroUnknown {
	_pointer := _self.ffiObject.incrementPointer("*ValueOrContainer")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalLoroUnknownINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_valueorcontainer_as_loro_unknown(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *ValueOrContainer) AsValue() *LoroValue {
	_pointer := _self.ffiObject.incrementPointer("*ValueOrContainer")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalLoroValueINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_valueorcontainer_as_value(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *ValueOrContainer) ContainerType() *ContainerType {
	_pointer := _self.ffiObject.incrementPointer("*ValueOrContainer")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalContainerTypeINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_valueorcontainer_container_type(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *ValueOrContainer) IsContainer() bool {
	_pointer := _self.ffiObject.incrementPointer("*ValueOrContainer")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_loro_fn_method_valueorcontainer_is_container(
			_pointer, _uniffiStatus)
	}))
}

func (_self *ValueOrContainer) IsValue() bool {
	_pointer := _self.ffiObject.incrementPointer("*ValueOrContainer")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_loro_fn_method_valueorcontainer_is_value(
			_pointer, _uniffiStatus)
	}))
}
func (object *ValueOrContainer) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterValueOrContainer struct{}

var FfiConverterValueOrContainerINSTANCE = FfiConverterValueOrContainer{}

func (c FfiConverterValueOrContainer) Lift(pointer unsafe.Pointer) *ValueOrContainer {
	result := &ValueOrContainer{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_loro_fn_clone_valueorcontainer(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_loro_fn_free_valueorcontainer(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*ValueOrContainer).Destroy)
	return result
}

func (c FfiConverterValueOrContainer) Read(reader io.Reader) *ValueOrContainer {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterValueOrContainer) Lower(value *ValueOrContainer) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*ValueOrContainer")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterValueOrContainer) Write(writer io.Writer, value *ValueOrContainer) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerValueOrContainer struct{}

func (_ FfiDestroyerValueOrContainer) Destroy(value *ValueOrContainer) {
	value.Destroy()
}

type VersionVectorInterface interface {
	Diff(rhs *VersionVector) VersionVectorDiff
	Encode() []byte
	Eq(other *VersionVector) bool
	ExtendToIncludeVv(other *VersionVector)
	GetLast(peer uint64) *int32
	GetMissingSpan(target *VersionVector) []IdSpan
	IncludesId(id Id) bool
	IncludesVv(other *VersionVector) bool
	IntersectSpan(target IdSpan) *CounterSpan
	Merge(other *VersionVector)
	PartialCmp(other *VersionVector) *Ordering
	SetEnd(id Id)
	SetLast(id Id)
}
type VersionVector struct {
	ffiObject FfiObject
}

func NewVersionVector() *VersionVector {
	return FfiConverterVersionVectorINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_constructor_versionvector_new(_uniffiStatus)
	}))
}

func VersionVectorDecode(bytes []byte) (*VersionVector, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_loro_fn_constructor_versionvector_decode(FfiConverterBytesINSTANCE.Lower(bytes), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *VersionVector
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterVersionVectorINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *VersionVector) Diff(rhs *VersionVector) VersionVectorDiff {
	_pointer := _self.ffiObject.incrementPointer("*VersionVector")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterVersionVectorDiffINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_versionvector_diff(
				_pointer, FfiConverterVersionVectorINSTANCE.Lower(rhs), _uniffiStatus),
		}
	}))
}

func (_self *VersionVector) Encode() []byte {
	_pointer := _self.ffiObject.incrementPointer("*VersionVector")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBytesINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_versionvector_encode(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *VersionVector) Eq(other *VersionVector) bool {
	_pointer := _self.ffiObject.incrementPointer("*VersionVector")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_loro_fn_method_versionvector_eq(
			_pointer, FfiConverterVersionVectorINSTANCE.Lower(other), _uniffiStatus)
	}))
}

func (_self *VersionVector) ExtendToIncludeVv(other *VersionVector) {
	_pointer := _self.ffiObject.incrementPointer("*VersionVector")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_versionvector_extend_to_include_vv(
			_pointer, FfiConverterVersionVectorINSTANCE.Lower(other), _uniffiStatus)
		return false
	})
}

func (_self *VersionVector) GetLast(peer uint64) *int32 {
	_pointer := _self.ffiObject.incrementPointer("*VersionVector")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalInt32INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_versionvector_get_last(
				_pointer, FfiConverterUint64INSTANCE.Lower(peer), _uniffiStatus),
		}
	}))
}

func (_self *VersionVector) GetMissingSpan(target *VersionVector) []IdSpan {
	_pointer := _self.ffiObject.incrementPointer("*VersionVector")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceIdSpanINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_versionvector_get_missing_span(
				_pointer, FfiConverterVersionVectorINSTANCE.Lower(target), _uniffiStatus),
		}
	}))
}

func (_self *VersionVector) IncludesId(id Id) bool {
	_pointer := _self.ffiObject.incrementPointer("*VersionVector")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_loro_fn_method_versionvector_includes_id(
			_pointer, FfiConverterIdINSTANCE.Lower(id), _uniffiStatus)
	}))
}

func (_self *VersionVector) IncludesVv(other *VersionVector) bool {
	_pointer := _self.ffiObject.incrementPointer("*VersionVector")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_loro_fn_method_versionvector_includes_vv(
			_pointer, FfiConverterVersionVectorINSTANCE.Lower(other), _uniffiStatus)
	}))
}

func (_self *VersionVector) IntersectSpan(target IdSpan) *CounterSpan {
	_pointer := _self.ffiObject.incrementPointer("*VersionVector")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalCounterSpanINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_versionvector_intersect_span(
				_pointer, FfiConverterIdSpanINSTANCE.Lower(target), _uniffiStatus),
		}
	}))
}

func (_self *VersionVector) Merge(other *VersionVector) {
	_pointer := _self.ffiObject.incrementPointer("*VersionVector")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_versionvector_merge(
			_pointer, FfiConverterVersionVectorINSTANCE.Lower(other), _uniffiStatus)
		return false
	})
}

func (_self *VersionVector) PartialCmp(other *VersionVector) *Ordering {
	_pointer := _self.ffiObject.incrementPointer("*VersionVector")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalOrderingINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_method_versionvector_partial_cmp(
				_pointer, FfiConverterVersionVectorINSTANCE.Lower(other), _uniffiStatus),
		}
	}))
}

func (_self *VersionVector) SetEnd(id Id) {
	_pointer := _self.ffiObject.incrementPointer("*VersionVector")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_versionvector_set_end(
			_pointer, FfiConverterIdINSTANCE.Lower(id), _uniffiStatus)
		return false
	})
}

func (_self *VersionVector) SetLast(id Id) {
	_pointer := _self.ffiObject.incrementPointer("*VersionVector")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_loro_fn_method_versionvector_set_last(
			_pointer, FfiConverterIdINSTANCE.Lower(id), _uniffiStatus)
		return false
	})
}
func (object *VersionVector) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterVersionVector struct{}

var FfiConverterVersionVectorINSTANCE = FfiConverterVersionVector{}

func (c FfiConverterVersionVector) Lift(pointer unsafe.Pointer) *VersionVector {
	result := &VersionVector{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_loro_fn_clone_versionvector(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_loro_fn_free_versionvector(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*VersionVector).Destroy)
	return result
}

func (c FfiConverterVersionVector) Read(reader io.Reader) *VersionVector {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterVersionVector) Lower(value *VersionVector) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*VersionVector")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterVersionVector) Write(writer io.Writer, value *VersionVector) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerVersionVector struct{}

func (_ FfiDestroyerVersionVector) Destroy(value *VersionVector) {
	value.Destroy()
}

type AbsolutePosition struct {
	Pos  uint32
	Side Side
}

func (r *AbsolutePosition) Destroy() {
	FfiDestroyerUint32{}.Destroy(r.Pos)
	FfiDestroyerSide{}.Destroy(r.Side)
}

type FfiConverterAbsolutePosition struct{}

var FfiConverterAbsolutePositionINSTANCE = FfiConverterAbsolutePosition{}

func (c FfiConverterAbsolutePosition) Lift(rb RustBufferI) AbsolutePosition {
	return LiftFromRustBuffer[AbsolutePosition](c, rb)
}

func (c FfiConverterAbsolutePosition) Read(reader io.Reader) AbsolutePosition {
	return AbsolutePosition{
		FfiConverterUint32INSTANCE.Read(reader),
		FfiConverterSideINSTANCE.Read(reader),
	}
}

func (c FfiConverterAbsolutePosition) Lower(value AbsolutePosition) C.RustBuffer {
	return LowerIntoRustBuffer[AbsolutePosition](c, value)
}

func (c FfiConverterAbsolutePosition) Write(writer io.Writer, value AbsolutePosition) {
	FfiConverterUint32INSTANCE.Write(writer, value.Pos)
	FfiConverterSideINSTANCE.Write(writer, value.Side)
}

type FfiDestroyerAbsolutePosition struct{}

func (_ FfiDestroyerAbsolutePosition) Destroy(value AbsolutePosition) {
	value.Destroy()
}

type AwarenessPeerUpdate struct {
	Updated []uint64
	Added   []uint64
}

func (r *AwarenessPeerUpdate) Destroy() {
	FfiDestroyerSequenceUint64{}.Destroy(r.Updated)
	FfiDestroyerSequenceUint64{}.Destroy(r.Added)
}

type FfiConverterAwarenessPeerUpdate struct{}

var FfiConverterAwarenessPeerUpdateINSTANCE = FfiConverterAwarenessPeerUpdate{}

func (c FfiConverterAwarenessPeerUpdate) Lift(rb RustBufferI) AwarenessPeerUpdate {
	return LiftFromRustBuffer[AwarenessPeerUpdate](c, rb)
}

func (c FfiConverterAwarenessPeerUpdate) Read(reader io.Reader) AwarenessPeerUpdate {
	return AwarenessPeerUpdate{
		FfiConverterSequenceUint64INSTANCE.Read(reader),
		FfiConverterSequenceUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterAwarenessPeerUpdate) Lower(value AwarenessPeerUpdate) C.RustBuffer {
	return LowerIntoRustBuffer[AwarenessPeerUpdate](c, value)
}

func (c FfiConverterAwarenessPeerUpdate) Write(writer io.Writer, value AwarenessPeerUpdate) {
	FfiConverterSequenceUint64INSTANCE.Write(writer, value.Updated)
	FfiConverterSequenceUint64INSTANCE.Write(writer, value.Added)
}

type FfiDestroyerAwarenessPeerUpdate struct{}

func (_ FfiDestroyerAwarenessPeerUpdate) Destroy(value AwarenessPeerUpdate) {
	value.Destroy()
}

type ChangeMeta struct {
	// Lamport timestamp of the Change
	Lamport uint32
	// The first Op id of the Change
	Id Id
	// [Unix time](https://en.wikipedia.org/wiki/Unix_time)
	// It is the number of seconds that have elapsed since 00:00:00 UTC on 1 January 1970.
	Timestamp int64
	// The commit message of the change
	Message *string
	// The dependencies of the first op of the change
	Deps *Frontiers
	// The total op num inside this change
	Len uint32
}

func (r *ChangeMeta) Destroy() {
	FfiDestroyerUint32{}.Destroy(r.Lamport)
	FfiDestroyerId{}.Destroy(r.Id)
	FfiDestroyerInt64{}.Destroy(r.Timestamp)
	FfiDestroyerOptionalString{}.Destroy(r.Message)
	FfiDestroyerFrontiers{}.Destroy(r.Deps)
	FfiDestroyerUint32{}.Destroy(r.Len)
}

type FfiConverterChangeMeta struct{}

var FfiConverterChangeMetaINSTANCE = FfiConverterChangeMeta{}

func (c FfiConverterChangeMeta) Lift(rb RustBufferI) ChangeMeta {
	return LiftFromRustBuffer[ChangeMeta](c, rb)
}

func (c FfiConverterChangeMeta) Read(reader io.Reader) ChangeMeta {
	return ChangeMeta{
		FfiConverterUint32INSTANCE.Read(reader),
		FfiConverterIdINSTANCE.Read(reader),
		FfiConverterInt64INSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterFrontiersINSTANCE.Read(reader),
		FfiConverterUint32INSTANCE.Read(reader),
	}
}

func (c FfiConverterChangeMeta) Lower(value ChangeMeta) C.RustBuffer {
	return LowerIntoRustBuffer[ChangeMeta](c, value)
}

func (c FfiConverterChangeMeta) Write(writer io.Writer, value ChangeMeta) {
	FfiConverterUint32INSTANCE.Write(writer, value.Lamport)
	FfiConverterIdINSTANCE.Write(writer, value.Id)
	FfiConverterInt64INSTANCE.Write(writer, value.Timestamp)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Message)
	FfiConverterFrontiersINSTANCE.Write(writer, value.Deps)
	FfiConverterUint32INSTANCE.Write(writer, value.Len)
}

type FfiDestroyerChangeMeta struct{}

func (_ FfiDestroyerChangeMeta) Destroy(value ChangeMeta) {
	value.Destroy()
}

type CommitOptions struct {
	Origin         *string
	ImmediateRenew bool
	Timestamp      *int64
	CommitMsg      *string
}

func (r *CommitOptions) Destroy() {
	FfiDestroyerOptionalString{}.Destroy(r.Origin)
	FfiDestroyerBool{}.Destroy(r.ImmediateRenew)
	FfiDestroyerOptionalInt64{}.Destroy(r.Timestamp)
	FfiDestroyerOptionalString{}.Destroy(r.CommitMsg)
}

type FfiConverterCommitOptions struct{}

var FfiConverterCommitOptionsINSTANCE = FfiConverterCommitOptions{}

func (c FfiConverterCommitOptions) Lift(rb RustBufferI) CommitOptions {
	return LiftFromRustBuffer[CommitOptions](c, rb)
}

func (c FfiConverterCommitOptions) Read(reader io.Reader) CommitOptions {
	return CommitOptions{
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterBoolINSTANCE.Read(reader),
		FfiConverterOptionalInt64INSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterCommitOptions) Lower(value CommitOptions) C.RustBuffer {
	return LowerIntoRustBuffer[CommitOptions](c, value)
}

func (c FfiConverterCommitOptions) Write(writer io.Writer, value CommitOptions) {
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Origin)
	FfiConverterBoolINSTANCE.Write(writer, value.ImmediateRenew)
	FfiConverterOptionalInt64INSTANCE.Write(writer, value.Timestamp)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.CommitMsg)
}

type FfiDestroyerCommitOptions struct{}

func (_ FfiDestroyerCommitOptions) Destroy(value CommitOptions) {
	value.Destroy()
}

// A diff of a container.
type ContainerDiff struct {
	// The target container id of the diff.
	Target ContainerId
	// The path of the diff.
	Path []PathItem
	// Whether the diff is from unknown container.
	IsUnknown bool
	// The diff
	Diff Diff
}

func (r *ContainerDiff) Destroy() {
	FfiDestroyerContainerId{}.Destroy(r.Target)
	FfiDestroyerSequencePathItem{}.Destroy(r.Path)
	FfiDestroyerBool{}.Destroy(r.IsUnknown)
	FfiDestroyerDiff{}.Destroy(r.Diff)
}

type FfiConverterContainerDiff struct{}

var FfiConverterContainerDiffINSTANCE = FfiConverterContainerDiff{}

func (c FfiConverterContainerDiff) Lift(rb RustBufferI) ContainerDiff {
	return LiftFromRustBuffer[ContainerDiff](c, rb)
}

func (c FfiConverterContainerDiff) Read(reader io.Reader) ContainerDiff {
	return ContainerDiff{
		FfiConverterContainerIdINSTANCE.Read(reader),
		FfiConverterSequencePathItemINSTANCE.Read(reader),
		FfiConverterBoolINSTANCE.Read(reader),
		FfiConverterDiffINSTANCE.Read(reader),
	}
}

func (c FfiConverterContainerDiff) Lower(value ContainerDiff) C.RustBuffer {
	return LowerIntoRustBuffer[ContainerDiff](c, value)
}

func (c FfiConverterContainerDiff) Write(writer io.Writer, value ContainerDiff) {
	FfiConverterContainerIdINSTANCE.Write(writer, value.Target)
	FfiConverterSequencePathItemINSTANCE.Write(writer, value.Path)
	FfiConverterBoolINSTANCE.Write(writer, value.IsUnknown)
	FfiConverterDiffINSTANCE.Write(writer, value.Diff)
}

type FfiDestroyerContainerDiff struct{}

func (_ FfiDestroyerContainerDiff) Destroy(value ContainerDiff) {
	value.Destroy()
}

type ContainerIdAndDiff struct {
	Cid  ContainerId
	Diff Diff
}

func (r *ContainerIdAndDiff) Destroy() {
	FfiDestroyerContainerId{}.Destroy(r.Cid)
	FfiDestroyerDiff{}.Destroy(r.Diff)
}

type FfiConverterContainerIdAndDiff struct{}

var FfiConverterContainerIdAndDiffINSTANCE = FfiConverterContainerIdAndDiff{}

func (c FfiConverterContainerIdAndDiff) Lift(rb RustBufferI) ContainerIdAndDiff {
	return LiftFromRustBuffer[ContainerIdAndDiff](c, rb)
}

func (c FfiConverterContainerIdAndDiff) Read(reader io.Reader) ContainerIdAndDiff {
	return ContainerIdAndDiff{
		FfiConverterContainerIdINSTANCE.Read(reader),
		FfiConverterDiffINSTANCE.Read(reader),
	}
}

func (c FfiConverterContainerIdAndDiff) Lower(value ContainerIdAndDiff) C.RustBuffer {
	return LowerIntoRustBuffer[ContainerIdAndDiff](c, value)
}

func (c FfiConverterContainerIdAndDiff) Write(writer io.Writer, value ContainerIdAndDiff) {
	FfiConverterContainerIdINSTANCE.Write(writer, value.Cid)
	FfiConverterDiffINSTANCE.Write(writer, value.Diff)
}

type FfiDestroyerContainerIdAndDiff struct{}

func (_ FfiDestroyerContainerIdAndDiff) Destroy(value ContainerIdAndDiff) {
	value.Destroy()
}

type ContainerPath struct {
	Id   ContainerId
	Path Index
}

func (r *ContainerPath) Destroy() {
	FfiDestroyerContainerId{}.Destroy(r.Id)
	FfiDestroyerIndex{}.Destroy(r.Path)
}

type FfiConverterContainerPath struct{}

var FfiConverterContainerPathINSTANCE = FfiConverterContainerPath{}

func (c FfiConverterContainerPath) Lift(rb RustBufferI) ContainerPath {
	return LiftFromRustBuffer[ContainerPath](c, rb)
}

func (c FfiConverterContainerPath) Read(reader io.Reader) ContainerPath {
	return ContainerPath{
		FfiConverterContainerIdINSTANCE.Read(reader),
		FfiConverterIndexINSTANCE.Read(reader),
	}
}

func (c FfiConverterContainerPath) Lower(value ContainerPath) C.RustBuffer {
	return LowerIntoRustBuffer[ContainerPath](c, value)
}

func (c FfiConverterContainerPath) Write(writer io.Writer, value ContainerPath) {
	FfiConverterContainerIdINSTANCE.Write(writer, value.Id)
	FfiConverterIndexINSTANCE.Write(writer, value.Path)
}

type FfiDestroyerContainerPath struct{}

func (_ FfiDestroyerContainerPath) Destroy(value ContainerPath) {
	value.Destroy()
}

type CounterSpan struct {
	Start int32
	End   int32
}

func (r *CounterSpan) Destroy() {
	FfiDestroyerInt32{}.Destroy(r.Start)
	FfiDestroyerInt32{}.Destroy(r.End)
}

type FfiConverterCounterSpan struct{}

var FfiConverterCounterSpanINSTANCE = FfiConverterCounterSpan{}

func (c FfiConverterCounterSpan) Lift(rb RustBufferI) CounterSpan {
	return LiftFromRustBuffer[CounterSpan](c, rb)
}

func (c FfiConverterCounterSpan) Read(reader io.Reader) CounterSpan {
	return CounterSpan{
		FfiConverterInt32INSTANCE.Read(reader),
		FfiConverterInt32INSTANCE.Read(reader),
	}
}

func (c FfiConverterCounterSpan) Lower(value CounterSpan) C.RustBuffer {
	return LowerIntoRustBuffer[CounterSpan](c, value)
}

func (c FfiConverterCounterSpan) Write(writer io.Writer, value CounterSpan) {
	FfiConverterInt32INSTANCE.Write(writer, value.Start)
	FfiConverterInt32INSTANCE.Write(writer, value.End)
}

type FfiDestroyerCounterSpan struct{}

func (_ FfiDestroyerCounterSpan) Destroy(value CounterSpan) {
	value.Destroy()
}

type CursorWithPos struct {
	Cursor *Cursor
	Pos    AbsolutePosition
}

func (r *CursorWithPos) Destroy() {
	FfiDestroyerCursor{}.Destroy(r.Cursor)
	FfiDestroyerAbsolutePosition{}.Destroy(r.Pos)
}

type FfiConverterCursorWithPos struct{}

var FfiConverterCursorWithPosINSTANCE = FfiConverterCursorWithPos{}

func (c FfiConverterCursorWithPos) Lift(rb RustBufferI) CursorWithPos {
	return LiftFromRustBuffer[CursorWithPos](c, rb)
}

func (c FfiConverterCursorWithPos) Read(reader io.Reader) CursorWithPos {
	return CursorWithPos{
		FfiConverterCursorINSTANCE.Read(reader),
		FfiConverterAbsolutePositionINSTANCE.Read(reader),
	}
}

func (c FfiConverterCursorWithPos) Lower(value CursorWithPos) C.RustBuffer {
	return LowerIntoRustBuffer[CursorWithPos](c, value)
}

func (c FfiConverterCursorWithPos) Write(writer io.Writer, value CursorWithPos) {
	FfiConverterCursorINSTANCE.Write(writer, value.Cursor)
	FfiConverterAbsolutePositionINSTANCE.Write(writer, value.Pos)
}

type FfiDestroyerCursorWithPos struct{}

func (_ FfiDestroyerCursorWithPos) Destroy(value CursorWithPos) {
	value.Destroy()
}

type DiffEvent struct {
	// How the event is triggered.
	TriggeredBy EventTriggerKind
	// The origin of the event.
	Origin string
	// The current receiver of the event.
	CurrentTarget *ContainerId
	// The diffs of the event.
	Events []ContainerDiff
}

func (r *DiffEvent) Destroy() {
	FfiDestroyerEventTriggerKind{}.Destroy(r.TriggeredBy)
	FfiDestroyerString{}.Destroy(r.Origin)
	FfiDestroyerOptionalContainerId{}.Destroy(r.CurrentTarget)
	FfiDestroyerSequenceContainerDiff{}.Destroy(r.Events)
}

type FfiConverterDiffEvent struct{}

var FfiConverterDiffEventINSTANCE = FfiConverterDiffEvent{}

func (c FfiConverterDiffEvent) Lift(rb RustBufferI) DiffEvent {
	return LiftFromRustBuffer[DiffEvent](c, rb)
}

func (c FfiConverterDiffEvent) Read(reader io.Reader) DiffEvent {
	return DiffEvent{
		FfiConverterEventTriggerKindINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterOptionalContainerIdINSTANCE.Read(reader),
		FfiConverterSequenceContainerDiffINSTANCE.Read(reader),
	}
}

func (c FfiConverterDiffEvent) Lower(value DiffEvent) C.RustBuffer {
	return LowerIntoRustBuffer[DiffEvent](c, value)
}

func (c FfiConverterDiffEvent) Write(writer io.Writer, value DiffEvent) {
	FfiConverterEventTriggerKindINSTANCE.Write(writer, value.TriggeredBy)
	FfiConverterStringINSTANCE.Write(writer, value.Origin)
	FfiConverterOptionalContainerIdINSTANCE.Write(writer, value.CurrentTarget)
	FfiConverterSequenceContainerDiffINSTANCE.Write(writer, value.Events)
}

type FfiDestroyerDiffEvent struct{}

func (_ FfiDestroyerDiffEvent) Destroy(value DiffEvent) {
	value.Destroy()
}

type EphemeralStoreEvent struct {
	By      EphemeralEventTrigger
	Added   []string
	Removed []string
	Updated []string
}

func (r *EphemeralStoreEvent) Destroy() {
	FfiDestroyerEphemeralEventTrigger{}.Destroy(r.By)
	FfiDestroyerSequenceString{}.Destroy(r.Added)
	FfiDestroyerSequenceString{}.Destroy(r.Removed)
	FfiDestroyerSequenceString{}.Destroy(r.Updated)
}

type FfiConverterEphemeralStoreEvent struct{}

var FfiConverterEphemeralStoreEventINSTANCE = FfiConverterEphemeralStoreEvent{}

func (c FfiConverterEphemeralStoreEvent) Lift(rb RustBufferI) EphemeralStoreEvent {
	return LiftFromRustBuffer[EphemeralStoreEvent](c, rb)
}

func (c FfiConverterEphemeralStoreEvent) Read(reader io.Reader) EphemeralStoreEvent {
	return EphemeralStoreEvent{
		FfiConverterEphemeralEventTriggerINSTANCE.Read(reader),
		FfiConverterSequenceStringINSTANCE.Read(reader),
		FfiConverterSequenceStringINSTANCE.Read(reader),
		FfiConverterSequenceStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterEphemeralStoreEvent) Lower(value EphemeralStoreEvent) C.RustBuffer {
	return LowerIntoRustBuffer[EphemeralStoreEvent](c, value)
}

func (c FfiConverterEphemeralStoreEvent) Write(writer io.Writer, value EphemeralStoreEvent) {
	FfiConverterEphemeralEventTriggerINSTANCE.Write(writer, value.By)
	FfiConverterSequenceStringINSTANCE.Write(writer, value.Added)
	FfiConverterSequenceStringINSTANCE.Write(writer, value.Removed)
	FfiConverterSequenceStringINSTANCE.Write(writer, value.Updated)
}

type FfiDestroyerEphemeralStoreEvent struct{}

func (_ FfiDestroyerEphemeralStoreEvent) Destroy(value EphemeralStoreEvent) {
	value.Destroy()
}

type FirstCommitFromPeerPayload struct {
	Peer uint64
}

func (r *FirstCommitFromPeerPayload) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.Peer)
}

type FfiConverterFirstCommitFromPeerPayload struct{}

var FfiConverterFirstCommitFromPeerPayloadINSTANCE = FfiConverterFirstCommitFromPeerPayload{}

func (c FfiConverterFirstCommitFromPeerPayload) Lift(rb RustBufferI) FirstCommitFromPeerPayload {
	return LiftFromRustBuffer[FirstCommitFromPeerPayload](c, rb)
}

func (c FfiConverterFirstCommitFromPeerPayload) Read(reader io.Reader) FirstCommitFromPeerPayload {
	return FirstCommitFromPeerPayload{
		FfiConverterUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterFirstCommitFromPeerPayload) Lower(value FirstCommitFromPeerPayload) C.RustBuffer {
	return LowerIntoRustBuffer[FirstCommitFromPeerPayload](c, value)
}

func (c FfiConverterFirstCommitFromPeerPayload) Write(writer io.Writer, value FirstCommitFromPeerPayload) {
	FfiConverterUint64INSTANCE.Write(writer, value.Peer)
}

type FfiDestroyerFirstCommitFromPeerPayload struct{}

func (_ FfiDestroyerFirstCommitFromPeerPayload) Destroy(value FirstCommitFromPeerPayload) {
	value.Destroy()
}

type FrontiersOrId struct {
	Frontiers **Frontiers
	Id        *Id
}

func (r *FrontiersOrId) Destroy() {
	FfiDestroyerOptionalFrontiers{}.Destroy(r.Frontiers)
	FfiDestroyerOptionalId{}.Destroy(r.Id)
}

type FfiConverterFrontiersOrId struct{}

var FfiConverterFrontiersOrIdINSTANCE = FfiConverterFrontiersOrId{}

func (c FfiConverterFrontiersOrId) Lift(rb RustBufferI) FrontiersOrId {
	return LiftFromRustBuffer[FrontiersOrId](c, rb)
}

func (c FfiConverterFrontiersOrId) Read(reader io.Reader) FrontiersOrId {
	return FrontiersOrId{
		FfiConverterOptionalFrontiersINSTANCE.Read(reader),
		FfiConverterOptionalIdINSTANCE.Read(reader),
	}
}

func (c FfiConverterFrontiersOrId) Lower(value FrontiersOrId) C.RustBuffer {
	return LowerIntoRustBuffer[FrontiersOrId](c, value)
}

func (c FfiConverterFrontiersOrId) Write(writer io.Writer, value FrontiersOrId) {
	FfiConverterOptionalFrontiersINSTANCE.Write(writer, value.Frontiers)
	FfiConverterOptionalIdINSTANCE.Write(writer, value.Id)
}

type FfiDestroyerFrontiersOrId struct{}

func (_ FfiDestroyerFrontiersOrId) Destroy(value FrontiersOrId) {
	value.Destroy()
}

type Id struct {
	Peer    uint64
	Counter int32
}

func (r *Id) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.Peer)
	FfiDestroyerInt32{}.Destroy(r.Counter)
}

type FfiConverterId struct{}

var FfiConverterIdINSTANCE = FfiConverterId{}

func (c FfiConverterId) Lift(rb RustBufferI) Id {
	return LiftFromRustBuffer[Id](c, rb)
}

func (c FfiConverterId) Read(reader io.Reader) Id {
	return Id{
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterInt32INSTANCE.Read(reader),
	}
}

func (c FfiConverterId) Lower(value Id) C.RustBuffer {
	return LowerIntoRustBuffer[Id](c, value)
}

func (c FfiConverterId) Write(writer io.Writer, value Id) {
	FfiConverterUint64INSTANCE.Write(writer, value.Peer)
	FfiConverterInt32INSTANCE.Write(writer, value.Counter)
}

type FfiDestroyerId struct{}

func (_ FfiDestroyerId) Destroy(value Id) {
	value.Destroy()
}

type IdLp struct {
	Lamport uint32
	Peer    uint64
}

func (r *IdLp) Destroy() {
	FfiDestroyerUint32{}.Destroy(r.Lamport)
	FfiDestroyerUint64{}.Destroy(r.Peer)
}

type FfiConverterIdLp struct{}

var FfiConverterIdLpINSTANCE = FfiConverterIdLp{}

func (c FfiConverterIdLp) Lift(rb RustBufferI) IdLp {
	return LiftFromRustBuffer[IdLp](c, rb)
}

func (c FfiConverterIdLp) Read(reader io.Reader) IdLp {
	return IdLp{
		FfiConverterUint32INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterIdLp) Lower(value IdLp) C.RustBuffer {
	return LowerIntoRustBuffer[IdLp](c, value)
}

func (c FfiConverterIdLp) Write(writer io.Writer, value IdLp) {
	FfiConverterUint32INSTANCE.Write(writer, value.Lamport)
	FfiConverterUint64INSTANCE.Write(writer, value.Peer)
}

type FfiDestroyerIdLp struct{}

func (_ FfiDestroyerIdLp) Destroy(value IdLp) {
	value.Destroy()
}

type IdSpan struct {
	Peer    uint64
	Counter CounterSpan
}

func (r *IdSpan) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.Peer)
	FfiDestroyerCounterSpan{}.Destroy(r.Counter)
}

type FfiConverterIdSpan struct{}

var FfiConverterIdSpanINSTANCE = FfiConverterIdSpan{}

func (c FfiConverterIdSpan) Lift(rb RustBufferI) IdSpan {
	return LiftFromRustBuffer[IdSpan](c, rb)
}

func (c FfiConverterIdSpan) Read(reader io.Reader) IdSpan {
	return IdSpan{
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterCounterSpanINSTANCE.Read(reader),
	}
}

func (c FfiConverterIdSpan) Lower(value IdSpan) C.RustBuffer {
	return LowerIntoRustBuffer[IdSpan](c, value)
}

func (c FfiConverterIdSpan) Write(writer io.Writer, value IdSpan) {
	FfiConverterUint64INSTANCE.Write(writer, value.Peer)
	FfiConverterCounterSpanINSTANCE.Write(writer, value.Counter)
}

type FfiDestroyerIdSpan struct{}

func (_ FfiDestroyerIdSpan) Destroy(value IdSpan) {
	value.Destroy()
}

type ImportBlobMetadata struct {
	// The partial start version vector.
	//
	// Import blob includes all the ops from `partial_start_vv` to `partial_end_vv`.
	// However, it does not constitute a complete version vector, as it only contains counters
	// from peers included within the import blob.
	PartialStartVv *VersionVector
	// The partial end version vector.
	//
	// Import blob includes all the ops from `partial_start_vv` to `partial_end_vv`.
	// However, it does not constitute a complete version vector, as it only contains counters
	// from peers included within the import blob.
	PartialEndVv   *VersionVector
	StartTimestamp int64
	StartFrontiers *Frontiers
	EndTimestamp   int64
	ChangeNum      uint32
	Mode           string
}

func (r *ImportBlobMetadata) Destroy() {
	FfiDestroyerVersionVector{}.Destroy(r.PartialStartVv)
	FfiDestroyerVersionVector{}.Destroy(r.PartialEndVv)
	FfiDestroyerInt64{}.Destroy(r.StartTimestamp)
	FfiDestroyerFrontiers{}.Destroy(r.StartFrontiers)
	FfiDestroyerInt64{}.Destroy(r.EndTimestamp)
	FfiDestroyerUint32{}.Destroy(r.ChangeNum)
	FfiDestroyerString{}.Destroy(r.Mode)
}

type FfiConverterImportBlobMetadata struct{}

var FfiConverterImportBlobMetadataINSTANCE = FfiConverterImportBlobMetadata{}

func (c FfiConverterImportBlobMetadata) Lift(rb RustBufferI) ImportBlobMetadata {
	return LiftFromRustBuffer[ImportBlobMetadata](c, rb)
}

func (c FfiConverterImportBlobMetadata) Read(reader io.Reader) ImportBlobMetadata {
	return ImportBlobMetadata{
		FfiConverterVersionVectorINSTANCE.Read(reader),
		FfiConverterVersionVectorINSTANCE.Read(reader),
		FfiConverterInt64INSTANCE.Read(reader),
		FfiConverterFrontiersINSTANCE.Read(reader),
		FfiConverterInt64INSTANCE.Read(reader),
		FfiConverterUint32INSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterImportBlobMetadata) Lower(value ImportBlobMetadata) C.RustBuffer {
	return LowerIntoRustBuffer[ImportBlobMetadata](c, value)
}

func (c FfiConverterImportBlobMetadata) Write(writer io.Writer, value ImportBlobMetadata) {
	FfiConverterVersionVectorINSTANCE.Write(writer, value.PartialStartVv)
	FfiConverterVersionVectorINSTANCE.Write(writer, value.PartialEndVv)
	FfiConverterInt64INSTANCE.Write(writer, value.StartTimestamp)
	FfiConverterFrontiersINSTANCE.Write(writer, value.StartFrontiers)
	FfiConverterInt64INSTANCE.Write(writer, value.EndTimestamp)
	FfiConverterUint32INSTANCE.Write(writer, value.ChangeNum)
	FfiConverterStringINSTANCE.Write(writer, value.Mode)
}

type FfiDestroyerImportBlobMetadata struct{}

func (_ FfiDestroyerImportBlobMetadata) Destroy(value ImportBlobMetadata) {
	value.Destroy()
}

type ImportStatus struct {
	Success map[uint64]CounterSpan
	Pending *map[uint64]CounterSpan
}

func (r *ImportStatus) Destroy() {
	FfiDestroyerMapUint64CounterSpan{}.Destroy(r.Success)
	FfiDestroyerOptionalMapUint64CounterSpan{}.Destroy(r.Pending)
}

type FfiConverterImportStatus struct{}

var FfiConverterImportStatusINSTANCE = FfiConverterImportStatus{}

func (c FfiConverterImportStatus) Lift(rb RustBufferI) ImportStatus {
	return LiftFromRustBuffer[ImportStatus](c, rb)
}

func (c FfiConverterImportStatus) Read(reader io.Reader) ImportStatus {
	return ImportStatus{
		FfiConverterMapUint64CounterSpanINSTANCE.Read(reader),
		FfiConverterOptionalMapUint64CounterSpanINSTANCE.Read(reader),
	}
}

func (c FfiConverterImportStatus) Lower(value ImportStatus) C.RustBuffer {
	return LowerIntoRustBuffer[ImportStatus](c, value)
}

func (c FfiConverterImportStatus) Write(writer io.Writer, value ImportStatus) {
	FfiConverterMapUint64CounterSpanINSTANCE.Write(writer, value.Success)
	FfiConverterOptionalMapUint64CounterSpanINSTANCE.Write(writer, value.Pending)
}

type FfiDestroyerImportStatus struct{}

func (_ FfiDestroyerImportStatus) Destroy(value ImportStatus) {
	value.Destroy()
}

type MapDelta struct {
	Updated map[string]**ValueOrContainer
}

func (r *MapDelta) Destroy() {
	FfiDestroyerMapStringOptionalValueOrContainer{}.Destroy(r.Updated)
}

type FfiConverterMapDelta struct{}

var FfiConverterMapDeltaINSTANCE = FfiConverterMapDelta{}

func (c FfiConverterMapDelta) Lift(rb RustBufferI) MapDelta {
	return LiftFromRustBuffer[MapDelta](c, rb)
}

func (c FfiConverterMapDelta) Read(reader io.Reader) MapDelta {
	return MapDelta{
		FfiConverterMapStringOptionalValueOrContainerINSTANCE.Read(reader),
	}
}

func (c FfiConverterMapDelta) Lower(value MapDelta) C.RustBuffer {
	return LowerIntoRustBuffer[MapDelta](c, value)
}

func (c FfiConverterMapDelta) Write(writer io.Writer, value MapDelta) {
	FfiConverterMapStringOptionalValueOrContainerINSTANCE.Write(writer, value.Updated)
}

type FfiDestroyerMapDelta struct{}

func (_ FfiDestroyerMapDelta) Destroy(value MapDelta) {
	value.Destroy()
}

type PathItem struct {
	Container ContainerId
	Index     Index
}

func (r *PathItem) Destroy() {
	FfiDestroyerContainerId{}.Destroy(r.Container)
	FfiDestroyerIndex{}.Destroy(r.Index)
}

type FfiConverterPathItem struct{}

var FfiConverterPathItemINSTANCE = FfiConverterPathItem{}

func (c FfiConverterPathItem) Lift(rb RustBufferI) PathItem {
	return LiftFromRustBuffer[PathItem](c, rb)
}

func (c FfiConverterPathItem) Read(reader io.Reader) PathItem {
	return PathItem{
		FfiConverterContainerIdINSTANCE.Read(reader),
		FfiConverterIndexINSTANCE.Read(reader),
	}
}

func (c FfiConverterPathItem) Lower(value PathItem) C.RustBuffer {
	return LowerIntoRustBuffer[PathItem](c, value)
}

func (c FfiConverterPathItem) Write(writer io.Writer, value PathItem) {
	FfiConverterContainerIdINSTANCE.Write(writer, value.Container)
	FfiConverterIndexINSTANCE.Write(writer, value.Index)
}

type FfiDestroyerPathItem struct{}

func (_ FfiDestroyerPathItem) Destroy(value PathItem) {
	value.Destroy()
}

type PeerInfo struct {
	State     LoroValue
	Counter   int32
	Timestamp int64
}

func (r *PeerInfo) Destroy() {
	FfiDestroyerLoroValue{}.Destroy(r.State)
	FfiDestroyerInt32{}.Destroy(r.Counter)
	FfiDestroyerInt64{}.Destroy(r.Timestamp)
}

type FfiConverterPeerInfo struct{}

var FfiConverterPeerInfoINSTANCE = FfiConverterPeerInfo{}

func (c FfiConverterPeerInfo) Lift(rb RustBufferI) PeerInfo {
	return LiftFromRustBuffer[PeerInfo](c, rb)
}

func (c FfiConverterPeerInfo) Read(reader io.Reader) PeerInfo {
	return PeerInfo{
		FfiConverterLoroValueINSTANCE.Read(reader),
		FfiConverterInt32INSTANCE.Read(reader),
		FfiConverterInt64INSTANCE.Read(reader),
	}
}

func (c FfiConverterPeerInfo) Lower(value PeerInfo) C.RustBuffer {
	return LowerIntoRustBuffer[PeerInfo](c, value)
}

func (c FfiConverterPeerInfo) Write(writer io.Writer, value PeerInfo) {
	FfiConverterLoroValueINSTANCE.Write(writer, value.State)
	FfiConverterInt32INSTANCE.Write(writer, value.Counter)
	FfiConverterInt64INSTANCE.Write(writer, value.Timestamp)
}

type FfiDestroyerPeerInfo struct{}

func (_ FfiDestroyerPeerInfo) Destroy(value PeerInfo) {
	value.Destroy()
}

type PosQueryResult struct {
	Update  **Cursor
	Current AbsolutePosition
}

func (r *PosQueryResult) Destroy() {
	FfiDestroyerOptionalCursor{}.Destroy(r.Update)
	FfiDestroyerAbsolutePosition{}.Destroy(r.Current)
}

type FfiConverterPosQueryResult struct{}

var FfiConverterPosQueryResultINSTANCE = FfiConverterPosQueryResult{}

func (c FfiConverterPosQueryResult) Lift(rb RustBufferI) PosQueryResult {
	return LiftFromRustBuffer[PosQueryResult](c, rb)
}

func (c FfiConverterPosQueryResult) Read(reader io.Reader) PosQueryResult {
	return PosQueryResult{
		FfiConverterOptionalCursorINSTANCE.Read(reader),
		FfiConverterAbsolutePositionINSTANCE.Read(reader),
	}
}

func (c FfiConverterPosQueryResult) Lower(value PosQueryResult) C.RustBuffer {
	return LowerIntoRustBuffer[PosQueryResult](c, value)
}

func (c FfiConverterPosQueryResult) Write(writer io.Writer, value PosQueryResult) {
	FfiConverterOptionalCursorINSTANCE.Write(writer, value.Update)
	FfiConverterAbsolutePositionINSTANCE.Write(writer, value.Current)
}

type FfiDestroyerPosQueryResult struct{}

func (_ FfiDestroyerPosQueryResult) Destroy(value PosQueryResult) {
	value.Destroy()
}

type PreCommitCallbackPayload struct {
	ChangeMeta ChangeMeta
	Origin     string
	Modifier   *ChangeModifier
}

func (r *PreCommitCallbackPayload) Destroy() {
	FfiDestroyerChangeMeta{}.Destroy(r.ChangeMeta)
	FfiDestroyerString{}.Destroy(r.Origin)
	FfiDestroyerChangeModifier{}.Destroy(r.Modifier)
}

type FfiConverterPreCommitCallbackPayload struct{}

var FfiConverterPreCommitCallbackPayloadINSTANCE = FfiConverterPreCommitCallbackPayload{}

func (c FfiConverterPreCommitCallbackPayload) Lift(rb RustBufferI) PreCommitCallbackPayload {
	return LiftFromRustBuffer[PreCommitCallbackPayload](c, rb)
}

func (c FfiConverterPreCommitCallbackPayload) Read(reader io.Reader) PreCommitCallbackPayload {
	return PreCommitCallbackPayload{
		FfiConverterChangeMetaINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterChangeModifierINSTANCE.Read(reader),
	}
}

func (c FfiConverterPreCommitCallbackPayload) Lower(value PreCommitCallbackPayload) C.RustBuffer {
	return LowerIntoRustBuffer[PreCommitCallbackPayload](c, value)
}

func (c FfiConverterPreCommitCallbackPayload) Write(writer io.Writer, value PreCommitCallbackPayload) {
	FfiConverterChangeMetaINSTANCE.Write(writer, value.ChangeMeta)
	FfiConverterStringINSTANCE.Write(writer, value.Origin)
	FfiConverterChangeModifierINSTANCE.Write(writer, value.Modifier)
}

type FfiDestroyerPreCommitCallbackPayload struct{}

func (_ FfiDestroyerPreCommitCallbackPayload) Destroy(value PreCommitCallbackPayload) {
	value.Destroy()
}

type StyleConfig struct {
	Expand ExpandType
}

func (r *StyleConfig) Destroy() {
	FfiDestroyerExpandType{}.Destroy(r.Expand)
}

type FfiConverterStyleConfig struct{}

var FfiConverterStyleConfigINSTANCE = FfiConverterStyleConfig{}

func (c FfiConverterStyleConfig) Lift(rb RustBufferI) StyleConfig {
	return LiftFromRustBuffer[StyleConfig](c, rb)
}

func (c FfiConverterStyleConfig) Read(reader io.Reader) StyleConfig {
	return StyleConfig{
		FfiConverterExpandTypeINSTANCE.Read(reader),
	}
}

func (c FfiConverterStyleConfig) Lower(value StyleConfig) C.RustBuffer {
	return LowerIntoRustBuffer[StyleConfig](c, value)
}

func (c FfiConverterStyleConfig) Write(writer io.Writer, value StyleConfig) {
	FfiConverterExpandTypeINSTANCE.Write(writer, value.Expand)
}

type FfiDestroyerStyleConfig struct{}

func (_ FfiDestroyerStyleConfig) Destroy(value StyleConfig) {
	value.Destroy()
}

type TreeDiff struct {
	Diff []TreeDiffItem
}

func (r *TreeDiff) Destroy() {
	FfiDestroyerSequenceTreeDiffItem{}.Destroy(r.Diff)
}

type FfiConverterTreeDiff struct{}

var FfiConverterTreeDiffINSTANCE = FfiConverterTreeDiff{}

func (c FfiConverterTreeDiff) Lift(rb RustBufferI) TreeDiff {
	return LiftFromRustBuffer[TreeDiff](c, rb)
}

func (c FfiConverterTreeDiff) Read(reader io.Reader) TreeDiff {
	return TreeDiff{
		FfiConverterSequenceTreeDiffItemINSTANCE.Read(reader),
	}
}

func (c FfiConverterTreeDiff) Lower(value TreeDiff) C.RustBuffer {
	return LowerIntoRustBuffer[TreeDiff](c, value)
}

func (c FfiConverterTreeDiff) Write(writer io.Writer, value TreeDiff) {
	FfiConverterSequenceTreeDiffItemINSTANCE.Write(writer, value.Diff)
}

type FfiDestroyerTreeDiff struct{}

func (_ FfiDestroyerTreeDiff) Destroy(value TreeDiff) {
	value.Destroy()
}

type TreeDiffItem struct {
	Target TreeId
	Action TreeExternalDiff
}

func (r *TreeDiffItem) Destroy() {
	FfiDestroyerTreeId{}.Destroy(r.Target)
	FfiDestroyerTreeExternalDiff{}.Destroy(r.Action)
}

type FfiConverterTreeDiffItem struct{}

var FfiConverterTreeDiffItemINSTANCE = FfiConverterTreeDiffItem{}

func (c FfiConverterTreeDiffItem) Lift(rb RustBufferI) TreeDiffItem {
	return LiftFromRustBuffer[TreeDiffItem](c, rb)
}

func (c FfiConverterTreeDiffItem) Read(reader io.Reader) TreeDiffItem {
	return TreeDiffItem{
		FfiConverterTreeIdINSTANCE.Read(reader),
		FfiConverterTreeExternalDiffINSTANCE.Read(reader),
	}
}

func (c FfiConverterTreeDiffItem) Lower(value TreeDiffItem) C.RustBuffer {
	return LowerIntoRustBuffer[TreeDiffItem](c, value)
}

func (c FfiConverterTreeDiffItem) Write(writer io.Writer, value TreeDiffItem) {
	FfiConverterTreeIdINSTANCE.Write(writer, value.Target)
	FfiConverterTreeExternalDiffINSTANCE.Write(writer, value.Action)
}

type FfiDestroyerTreeDiffItem struct{}

func (_ FfiDestroyerTreeDiffItem) Destroy(value TreeDiffItem) {
	value.Destroy()
}

type TreeId struct {
	Peer    uint64
	Counter int32
}

func (r *TreeId) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.Peer)
	FfiDestroyerInt32{}.Destroy(r.Counter)
}

type FfiConverterTreeId struct{}

var FfiConverterTreeIdINSTANCE = FfiConverterTreeId{}

func (c FfiConverterTreeId) Lift(rb RustBufferI) TreeId {
	return LiftFromRustBuffer[TreeId](c, rb)
}

func (c FfiConverterTreeId) Read(reader io.Reader) TreeId {
	return TreeId{
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterInt32INSTANCE.Read(reader),
	}
}

func (c FfiConverterTreeId) Lower(value TreeId) C.RustBuffer {
	return LowerIntoRustBuffer[TreeId](c, value)
}

func (c FfiConverterTreeId) Write(writer io.Writer, value TreeId) {
	FfiConverterUint64INSTANCE.Write(writer, value.Peer)
	FfiConverterInt32INSTANCE.Write(writer, value.Counter)
}

type FfiDestroyerTreeId struct{}

func (_ FfiDestroyerTreeId) Destroy(value TreeId) {
	value.Destroy()
}

type UndoItemMeta struct {
	Value   LoroValue
	Cursors []CursorWithPos
}

func (r *UndoItemMeta) Destroy() {
	FfiDestroyerLoroValue{}.Destroy(r.Value)
	FfiDestroyerSequenceCursorWithPos{}.Destroy(r.Cursors)
}

type FfiConverterUndoItemMeta struct{}

var FfiConverterUndoItemMetaINSTANCE = FfiConverterUndoItemMeta{}

func (c FfiConverterUndoItemMeta) Lift(rb RustBufferI) UndoItemMeta {
	return LiftFromRustBuffer[UndoItemMeta](c, rb)
}

func (c FfiConverterUndoItemMeta) Read(reader io.Reader) UndoItemMeta {
	return UndoItemMeta{
		FfiConverterLoroValueINSTANCE.Read(reader),
		FfiConverterSequenceCursorWithPosINSTANCE.Read(reader),
	}
}

func (c FfiConverterUndoItemMeta) Lower(value UndoItemMeta) C.RustBuffer {
	return LowerIntoRustBuffer[UndoItemMeta](c, value)
}

func (c FfiConverterUndoItemMeta) Write(writer io.Writer, value UndoItemMeta) {
	FfiConverterLoroValueINSTANCE.Write(writer, value.Value)
	FfiConverterSequenceCursorWithPosINSTANCE.Write(writer, value.Cursors)
}

type FfiDestroyerUndoItemMeta struct{}

func (_ FfiDestroyerUndoItemMeta) Destroy(value UndoItemMeta) {
	value.Destroy()
}

type UpdateOptions struct {
	TimeoutMs      *float64
	UseRefinedDiff bool
}

func (r *UpdateOptions) Destroy() {
	FfiDestroyerOptionalFloat64{}.Destroy(r.TimeoutMs)
	FfiDestroyerBool{}.Destroy(r.UseRefinedDiff)
}

type FfiConverterUpdateOptions struct{}

var FfiConverterUpdateOptionsINSTANCE = FfiConverterUpdateOptions{}

func (c FfiConverterUpdateOptions) Lift(rb RustBufferI) UpdateOptions {
	return LiftFromRustBuffer[UpdateOptions](c, rb)
}

func (c FfiConverterUpdateOptions) Read(reader io.Reader) UpdateOptions {
	return UpdateOptions{
		FfiConverterOptionalFloat64INSTANCE.Read(reader),
		FfiConverterBoolINSTANCE.Read(reader),
	}
}

func (c FfiConverterUpdateOptions) Lower(value UpdateOptions) C.RustBuffer {
	return LowerIntoRustBuffer[UpdateOptions](c, value)
}

func (c FfiConverterUpdateOptions) Write(writer io.Writer, value UpdateOptions) {
	FfiConverterOptionalFloat64INSTANCE.Write(writer, value.TimeoutMs)
	FfiConverterBoolINSTANCE.Write(writer, value.UseRefinedDiff)
}

type FfiDestroyerUpdateOptions struct{}

func (_ FfiDestroyerUpdateOptions) Destroy(value UpdateOptions) {
	value.Destroy()
}

type VersionVectorDiff struct {
	// need to add these spans to move from right to left
	Retreat map[uint64]CounterSpan
	// need to add these spans to move from left to right
	Forward map[uint64]CounterSpan
}

func (r *VersionVectorDiff) Destroy() {
	FfiDestroyerMapUint64CounterSpan{}.Destroy(r.Retreat)
	FfiDestroyerMapUint64CounterSpan{}.Destroy(r.Forward)
}

type FfiConverterVersionVectorDiff struct{}

var FfiConverterVersionVectorDiffINSTANCE = FfiConverterVersionVectorDiff{}

func (c FfiConverterVersionVectorDiff) Lift(rb RustBufferI) VersionVectorDiff {
	return LiftFromRustBuffer[VersionVectorDiff](c, rb)
}

func (c FfiConverterVersionVectorDiff) Read(reader io.Reader) VersionVectorDiff {
	return VersionVectorDiff{
		FfiConverterMapUint64CounterSpanINSTANCE.Read(reader),
		FfiConverterMapUint64CounterSpanINSTANCE.Read(reader),
	}
}

func (c FfiConverterVersionVectorDiff) Lower(value VersionVectorDiff) C.RustBuffer {
	return LowerIntoRustBuffer[VersionVectorDiff](c, value)
}

func (c FfiConverterVersionVectorDiff) Write(writer io.Writer, value VersionVectorDiff) {
	FfiConverterMapUint64CounterSpanINSTANCE.Write(writer, value.Retreat)
	FfiConverterMapUint64CounterSpanINSTANCE.Write(writer, value.Forward)
}

type FfiDestroyerVersionVectorDiff struct{}

func (_ FfiDestroyerVersionVectorDiff) Destroy(value VersionVectorDiff) {
	value.Destroy()
}

type CannotFindRelativePosition struct {
	err error
}

// Convience method to turn *CannotFindRelativePosition into error
// Avoiding treating nil pointer as non nil error interface
func (err *CannotFindRelativePosition) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err CannotFindRelativePosition) Error() string {
	return fmt.Sprintf("CannotFindRelativePosition: %s", err.err.Error())
}

func (err CannotFindRelativePosition) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrCannotFindRelativePositionContainerDeleted = fmt.Errorf("CannotFindRelativePositionContainerDeleted")
var ErrCannotFindRelativePositionHistoryCleared = fmt.Errorf("CannotFindRelativePositionHistoryCleared")
var ErrCannotFindRelativePositionIdNotFound = fmt.Errorf("CannotFindRelativePositionIdNotFound")

// Variant structs
type CannotFindRelativePositionContainerDeleted struct {
	message string
}

func NewCannotFindRelativePositionContainerDeleted() *CannotFindRelativePosition {
	return &CannotFindRelativePosition{err: &CannotFindRelativePositionContainerDeleted{}}
}

func (e CannotFindRelativePositionContainerDeleted) destroy() {
}

func (err CannotFindRelativePositionContainerDeleted) Error() string {
	return fmt.Sprintf("ContainerDeleted: %s", err.message)
}

func (self CannotFindRelativePositionContainerDeleted) Is(target error) bool {
	return target == ErrCannotFindRelativePositionContainerDeleted
}

type CannotFindRelativePositionHistoryCleared struct {
	message string
}

func NewCannotFindRelativePositionHistoryCleared() *CannotFindRelativePosition {
	return &CannotFindRelativePosition{err: &CannotFindRelativePositionHistoryCleared{}}
}

func (e CannotFindRelativePositionHistoryCleared) destroy() {
}

func (err CannotFindRelativePositionHistoryCleared) Error() string {
	return fmt.Sprintf("HistoryCleared: %s", err.message)
}

func (self CannotFindRelativePositionHistoryCleared) Is(target error) bool {
	return target == ErrCannotFindRelativePositionHistoryCleared
}

type CannotFindRelativePositionIdNotFound struct {
	message string
}

func NewCannotFindRelativePositionIdNotFound() *CannotFindRelativePosition {
	return &CannotFindRelativePosition{err: &CannotFindRelativePositionIdNotFound{}}
}

func (e CannotFindRelativePositionIdNotFound) destroy() {
}

func (err CannotFindRelativePositionIdNotFound) Error() string {
	return fmt.Sprintf("IdNotFound: %s", err.message)
}

func (self CannotFindRelativePositionIdNotFound) Is(target error) bool {
	return target == ErrCannotFindRelativePositionIdNotFound
}

type FfiConverterCannotFindRelativePosition struct{}

var FfiConverterCannotFindRelativePositionINSTANCE = FfiConverterCannotFindRelativePosition{}

func (c FfiConverterCannotFindRelativePosition) Lift(eb RustBufferI) *CannotFindRelativePosition {
	return LiftFromRustBuffer[*CannotFindRelativePosition](c, eb)
}

func (c FfiConverterCannotFindRelativePosition) Lower(value *CannotFindRelativePosition) C.RustBuffer {
	return LowerIntoRustBuffer[*CannotFindRelativePosition](c, value)
}

func (c FfiConverterCannotFindRelativePosition) Read(reader io.Reader) *CannotFindRelativePosition {
	errorID := readUint32(reader)

	message := FfiConverterStringINSTANCE.Read(reader)
	switch errorID {
	case 1:
		return &CannotFindRelativePosition{&CannotFindRelativePositionContainerDeleted{message}}
	case 2:
		return &CannotFindRelativePosition{&CannotFindRelativePositionHistoryCleared{message}}
	case 3:
		return &CannotFindRelativePosition{&CannotFindRelativePositionIdNotFound{message}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterCannotFindRelativePosition.Read()", errorID))
	}

}

func (c FfiConverterCannotFindRelativePosition) Write(writer io.Writer, value *CannotFindRelativePosition) {
	switch variantValue := value.err.(type) {
	case *CannotFindRelativePositionContainerDeleted:
		writeInt32(writer, 1)
	case *CannotFindRelativePositionHistoryCleared:
		writeInt32(writer, 2)
	case *CannotFindRelativePositionIdNotFound:
		writeInt32(writer, 3)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterCannotFindRelativePosition.Write", value))
	}
}

type FfiDestroyerCannotFindRelativePosition struct{}

func (_ FfiDestroyerCannotFindRelativePosition) Destroy(value *CannotFindRelativePosition) {
	switch variantValue := value.err.(type) {
	case CannotFindRelativePositionContainerDeleted:
		variantValue.destroy()
	case CannotFindRelativePositionHistoryCleared:
		variantValue.destroy()
	case CannotFindRelativePositionIdNotFound:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerCannotFindRelativePosition.Destroy", value))
	}
}

type ChangeTravelError struct {
	err error
}

// Convience method to turn *ChangeTravelError into error
// Avoiding treating nil pointer as non nil error interface
func (err *ChangeTravelError) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err ChangeTravelError) Error() string {
	return fmt.Sprintf("ChangeTravelError: %s", err.err.Error())
}

func (err ChangeTravelError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrChangeTravelErrorTargetIdNotFound = fmt.Errorf("ChangeTravelErrorTargetIdNotFound")
var ErrChangeTravelErrorTargetVersionNotIncluded = fmt.Errorf("ChangeTravelErrorTargetVersionNotIncluded")

// Variant structs
type ChangeTravelErrorTargetIdNotFound struct {
	message string
}

func NewChangeTravelErrorTargetIdNotFound() *ChangeTravelError {
	return &ChangeTravelError{err: &ChangeTravelErrorTargetIdNotFound{}}
}

func (e ChangeTravelErrorTargetIdNotFound) destroy() {
}

func (err ChangeTravelErrorTargetIdNotFound) Error() string {
	return fmt.Sprintf("TargetIdNotFound: %s", err.message)
}

func (self ChangeTravelErrorTargetIdNotFound) Is(target error) bool {
	return target == ErrChangeTravelErrorTargetIdNotFound
}

type ChangeTravelErrorTargetVersionNotIncluded struct {
	message string
}

func NewChangeTravelErrorTargetVersionNotIncluded() *ChangeTravelError {
	return &ChangeTravelError{err: &ChangeTravelErrorTargetVersionNotIncluded{}}
}

func (e ChangeTravelErrorTargetVersionNotIncluded) destroy() {
}

func (err ChangeTravelErrorTargetVersionNotIncluded) Error() string {
	return fmt.Sprintf("TargetVersionNotIncluded: %s", err.message)
}

func (self ChangeTravelErrorTargetVersionNotIncluded) Is(target error) bool {
	return target == ErrChangeTravelErrorTargetVersionNotIncluded
}

type FfiConverterChangeTravelError struct{}

var FfiConverterChangeTravelErrorINSTANCE = FfiConverterChangeTravelError{}

func (c FfiConverterChangeTravelError) Lift(eb RustBufferI) *ChangeTravelError {
	return LiftFromRustBuffer[*ChangeTravelError](c, eb)
}

func (c FfiConverterChangeTravelError) Lower(value *ChangeTravelError) C.RustBuffer {
	return LowerIntoRustBuffer[*ChangeTravelError](c, value)
}

func (c FfiConverterChangeTravelError) Read(reader io.Reader) *ChangeTravelError {
	errorID := readUint32(reader)

	message := FfiConverterStringINSTANCE.Read(reader)
	switch errorID {
	case 1:
		return &ChangeTravelError{&ChangeTravelErrorTargetIdNotFound{message}}
	case 2:
		return &ChangeTravelError{&ChangeTravelErrorTargetVersionNotIncluded{message}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterChangeTravelError.Read()", errorID))
	}

}

func (c FfiConverterChangeTravelError) Write(writer io.Writer, value *ChangeTravelError) {
	switch variantValue := value.err.(type) {
	case *ChangeTravelErrorTargetIdNotFound:
		writeInt32(writer, 1)
	case *ChangeTravelErrorTargetVersionNotIncluded:
		writeInt32(writer, 2)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterChangeTravelError.Write", value))
	}
}

type FfiDestroyerChangeTravelError struct{}

func (_ FfiDestroyerChangeTravelError) Destroy(value *ChangeTravelError) {
	switch variantValue := value.err.(type) {
	case ChangeTravelErrorTargetIdNotFound:
		variantValue.destroy()
	case ChangeTravelErrorTargetVersionNotIncluded:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerChangeTravelError.Destroy", value))
	}
}

type ContainerId interface {
	Destroy()
}
type ContainerIdRoot struct {
	Name          string
	ContainerType ContainerType
}

func (e ContainerIdRoot) Destroy() {
	FfiDestroyerString{}.Destroy(e.Name)
	FfiDestroyerContainerType{}.Destroy(e.ContainerType)
}

type ContainerIdNormal struct {
	Peer          uint64
	Counter       int32
	ContainerType ContainerType
}

func (e ContainerIdNormal) Destroy() {
	FfiDestroyerUint64{}.Destroy(e.Peer)
	FfiDestroyerInt32{}.Destroy(e.Counter)
	FfiDestroyerContainerType{}.Destroy(e.ContainerType)
}

type FfiConverterContainerId struct{}

var FfiConverterContainerIdINSTANCE = FfiConverterContainerId{}

func (c FfiConverterContainerId) Lift(rb RustBufferI) ContainerId {
	return LiftFromRustBuffer[ContainerId](c, rb)
}

func (c FfiConverterContainerId) Lower(value ContainerId) C.RustBuffer {
	return LowerIntoRustBuffer[ContainerId](c, value)
}
func (FfiConverterContainerId) Read(reader io.Reader) ContainerId {
	id := readInt32(reader)
	switch id {
	case 1:
		return ContainerIdRoot{
			FfiConverterStringINSTANCE.Read(reader),
			FfiConverterContainerTypeINSTANCE.Read(reader),
		}
	case 2:
		return ContainerIdNormal{
			FfiConverterUint64INSTANCE.Read(reader),
			FfiConverterInt32INSTANCE.Read(reader),
			FfiConverterContainerTypeINSTANCE.Read(reader),
		}
	default:
		panic(fmt.Sprintf("invalid enum value %v in FfiConverterContainerId.Read()", id))
	}
}

func (FfiConverterContainerId) Write(writer io.Writer, value ContainerId) {
	switch variant_value := value.(type) {
	case ContainerIdRoot:
		writeInt32(writer, 1)
		FfiConverterStringINSTANCE.Write(writer, variant_value.Name)
		FfiConverterContainerTypeINSTANCE.Write(writer, variant_value.ContainerType)
	case ContainerIdNormal:
		writeInt32(writer, 2)
		FfiConverterUint64INSTANCE.Write(writer, variant_value.Peer)
		FfiConverterInt32INSTANCE.Write(writer, variant_value.Counter)
		FfiConverterContainerTypeINSTANCE.Write(writer, variant_value.ContainerType)
	default:
		_ = variant_value
		panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterContainerId.Write", value))
	}
}

type FfiDestroyerContainerId struct{}

func (_ FfiDestroyerContainerId) Destroy(value ContainerId) {
	value.Destroy()
}

type ContainerType interface {
	Destroy()
}
type ContainerTypeText struct {
}

func (e ContainerTypeText) Destroy() {
}

type ContainerTypeMap struct {
}

func (e ContainerTypeMap) Destroy() {
}

type ContainerTypeList struct {
}

func (e ContainerTypeList) Destroy() {
}

type ContainerTypeMovableList struct {
}

func (e ContainerTypeMovableList) Destroy() {
}

type ContainerTypeTree struct {
}

func (e ContainerTypeTree) Destroy() {
}

type ContainerTypeCounter struct {
}

func (e ContainerTypeCounter) Destroy() {
}

type ContainerTypeUnknown struct {
	Kind uint8
}

func (e ContainerTypeUnknown) Destroy() {
	FfiDestroyerUint8{}.Destroy(e.Kind)
}

type FfiConverterContainerType struct{}

var FfiConverterContainerTypeINSTANCE = FfiConverterContainerType{}

func (c FfiConverterContainerType) Lift(rb RustBufferI) ContainerType {
	return LiftFromRustBuffer[ContainerType](c, rb)
}

func (c FfiConverterContainerType) Lower(value ContainerType) C.RustBuffer {
	return LowerIntoRustBuffer[ContainerType](c, value)
}
func (FfiConverterContainerType) Read(reader io.Reader) ContainerType {
	id := readInt32(reader)
	switch id {
	case 1:
		return ContainerTypeText{}
	case 2:
		return ContainerTypeMap{}
	case 3:
		return ContainerTypeList{}
	case 4:
		return ContainerTypeMovableList{}
	case 5:
		return ContainerTypeTree{}
	case 6:
		return ContainerTypeCounter{}
	case 7:
		return ContainerTypeUnknown{
			FfiConverterUint8INSTANCE.Read(reader),
		}
	default:
		panic(fmt.Sprintf("invalid enum value %v in FfiConverterContainerType.Read()", id))
	}
}

func (FfiConverterContainerType) Write(writer io.Writer, value ContainerType) {
	switch variant_value := value.(type) {
	case ContainerTypeText:
		writeInt32(writer, 1)
	case ContainerTypeMap:
		writeInt32(writer, 2)
	case ContainerTypeList:
		writeInt32(writer, 3)
	case ContainerTypeMovableList:
		writeInt32(writer, 4)
	case ContainerTypeTree:
		writeInt32(writer, 5)
	case ContainerTypeCounter:
		writeInt32(writer, 6)
	case ContainerTypeUnknown:
		writeInt32(writer, 7)
		FfiConverterUint8INSTANCE.Write(writer, variant_value.Kind)
	default:
		_ = variant_value
		panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterContainerType.Write", value))
	}
}

type FfiDestroyerContainerType struct{}

func (_ FfiDestroyerContainerType) Destroy(value ContainerType) {
	value.Destroy()
}

type Diff interface {
	Destroy()
}
type DiffList struct {
	Diff []ListDiffItem
}

func (e DiffList) Destroy() {
	FfiDestroyerSequenceListDiffItem{}.Destroy(e.Diff)
}

type DiffText struct {
	Diff []TextDelta
}

func (e DiffText) Destroy() {
	FfiDestroyerSequenceTextDelta{}.Destroy(e.Diff)
}

type DiffMap struct {
	Diff MapDelta
}

func (e DiffMap) Destroy() {
	FfiDestroyerMapDelta{}.Destroy(e.Diff)
}

type DiffTree struct {
	Diff TreeDiff
}

func (e DiffTree) Destroy() {
	FfiDestroyerTreeDiff{}.Destroy(e.Diff)
}

type DiffCounter struct {
	Diff float64
}

func (e DiffCounter) Destroy() {
	FfiDestroyerFloat64{}.Destroy(e.Diff)
}

type DiffUnknown struct {
}

func (e DiffUnknown) Destroy() {
}

type FfiConverterDiff struct{}

var FfiConverterDiffINSTANCE = FfiConverterDiff{}

func (c FfiConverterDiff) Lift(rb RustBufferI) Diff {
	return LiftFromRustBuffer[Diff](c, rb)
}

func (c FfiConverterDiff) Lower(value Diff) C.RustBuffer {
	return LowerIntoRustBuffer[Diff](c, value)
}
func (FfiConverterDiff) Read(reader io.Reader) Diff {
	id := readInt32(reader)
	switch id {
	case 1:
		return DiffList{
			FfiConverterSequenceListDiffItemINSTANCE.Read(reader),
		}
	case 2:
		return DiffText{
			FfiConverterSequenceTextDeltaINSTANCE.Read(reader),
		}
	case 3:
		return DiffMap{
			FfiConverterMapDeltaINSTANCE.Read(reader),
		}
	case 4:
		return DiffTree{
			FfiConverterTreeDiffINSTANCE.Read(reader),
		}
	case 5:
		return DiffCounter{
			FfiConverterFloat64INSTANCE.Read(reader),
		}
	case 6:
		return DiffUnknown{}
	default:
		panic(fmt.Sprintf("invalid enum value %v in FfiConverterDiff.Read()", id))
	}
}

func (FfiConverterDiff) Write(writer io.Writer, value Diff) {
	switch variant_value := value.(type) {
	case DiffList:
		writeInt32(writer, 1)
		FfiConverterSequenceListDiffItemINSTANCE.Write(writer, variant_value.Diff)
	case DiffText:
		writeInt32(writer, 2)
		FfiConverterSequenceTextDeltaINSTANCE.Write(writer, variant_value.Diff)
	case DiffMap:
		writeInt32(writer, 3)
		FfiConverterMapDeltaINSTANCE.Write(writer, variant_value.Diff)
	case DiffTree:
		writeInt32(writer, 4)
		FfiConverterTreeDiffINSTANCE.Write(writer, variant_value.Diff)
	case DiffCounter:
		writeInt32(writer, 5)
		FfiConverterFloat64INSTANCE.Write(writer, variant_value.Diff)
	case DiffUnknown:
		writeInt32(writer, 6)
	default:
		_ = variant_value
		panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterDiff.Write", value))
	}
}

type FfiDestroyerDiff struct{}

func (_ FfiDestroyerDiff) Destroy(value Diff) {
	value.Destroy()
}

type EphemeralEventTrigger uint

const (
	EphemeralEventTriggerLocal   EphemeralEventTrigger = 1
	EphemeralEventTriggerImport  EphemeralEventTrigger = 2
	EphemeralEventTriggerTimeout EphemeralEventTrigger = 3
)

type FfiConverterEphemeralEventTrigger struct{}

var FfiConverterEphemeralEventTriggerINSTANCE = FfiConverterEphemeralEventTrigger{}

func (c FfiConverterEphemeralEventTrigger) Lift(rb RustBufferI) EphemeralEventTrigger {
	return LiftFromRustBuffer[EphemeralEventTrigger](c, rb)
}

func (c FfiConverterEphemeralEventTrigger) Lower(value EphemeralEventTrigger) C.RustBuffer {
	return LowerIntoRustBuffer[EphemeralEventTrigger](c, value)
}
func (FfiConverterEphemeralEventTrigger) Read(reader io.Reader) EphemeralEventTrigger {
	id := readInt32(reader)
	return EphemeralEventTrigger(id)
}

func (FfiConverterEphemeralEventTrigger) Write(writer io.Writer, value EphemeralEventTrigger) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerEphemeralEventTrigger struct{}

func (_ FfiDestroyerEphemeralEventTrigger) Destroy(value EphemeralEventTrigger) {
}

// The kind of the event trigger.
type EventTriggerKind uint

const (
	// The event is triggered by a local transaction.
	EventTriggerKindLocal EventTriggerKind = 1
	// The event is triggered by importing
	EventTriggerKindImport EventTriggerKind = 2
	// The event is triggered by checkout
	EventTriggerKindCheckout EventTriggerKind = 3
)

type FfiConverterEventTriggerKind struct{}

var FfiConverterEventTriggerKindINSTANCE = FfiConverterEventTriggerKind{}

func (c FfiConverterEventTriggerKind) Lift(rb RustBufferI) EventTriggerKind {
	return LiftFromRustBuffer[EventTriggerKind](c, rb)
}

func (c FfiConverterEventTriggerKind) Lower(value EventTriggerKind) C.RustBuffer {
	return LowerIntoRustBuffer[EventTriggerKind](c, value)
}
func (FfiConverterEventTriggerKind) Read(reader io.Reader) EventTriggerKind {
	id := readInt32(reader)
	return EventTriggerKind(id)
}

func (FfiConverterEventTriggerKind) Write(writer io.Writer, value EventTriggerKind) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerEventTriggerKind struct{}

func (_ FfiDestroyerEventTriggerKind) Destroy(value EventTriggerKind) {
}

type ExpandType uint

const (
	ExpandTypeBefore ExpandType = 1
	ExpandTypeAfter  ExpandType = 2
	ExpandTypeBoth   ExpandType = 3
	ExpandTypeNone   ExpandType = 4
)

type FfiConverterExpandType struct{}

var FfiConverterExpandTypeINSTANCE = FfiConverterExpandType{}

func (c FfiConverterExpandType) Lift(rb RustBufferI) ExpandType {
	return LiftFromRustBuffer[ExpandType](c, rb)
}

func (c FfiConverterExpandType) Lower(value ExpandType) C.RustBuffer {
	return LowerIntoRustBuffer[ExpandType](c, value)
}
func (FfiConverterExpandType) Read(reader io.Reader) ExpandType {
	id := readInt32(reader)
	return ExpandType(id)
}

func (FfiConverterExpandType) Write(writer io.Writer, value ExpandType) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerExpandType struct{}

func (_ FfiDestroyerExpandType) Destroy(value ExpandType) {
}

type Index interface {
	Destroy()
}
type IndexKey struct {
	Key string
}

func (e IndexKey) Destroy() {
	FfiDestroyerString{}.Destroy(e.Key)
}

type IndexSeq struct {
	Index uint32
}

func (e IndexSeq) Destroy() {
	FfiDestroyerUint32{}.Destroy(e.Index)
}

type IndexNode struct {
	Target TreeId
}

func (e IndexNode) Destroy() {
	FfiDestroyerTreeId{}.Destroy(e.Target)
}

type FfiConverterIndex struct{}

var FfiConverterIndexINSTANCE = FfiConverterIndex{}

func (c FfiConverterIndex) Lift(rb RustBufferI) Index {
	return LiftFromRustBuffer[Index](c, rb)
}

func (c FfiConverterIndex) Lower(value Index) C.RustBuffer {
	return LowerIntoRustBuffer[Index](c, value)
}
func (FfiConverterIndex) Read(reader io.Reader) Index {
	id := readInt32(reader)
	switch id {
	case 1:
		return IndexKey{
			FfiConverterStringINSTANCE.Read(reader),
		}
	case 2:
		return IndexSeq{
			FfiConverterUint32INSTANCE.Read(reader),
		}
	case 3:
		return IndexNode{
			FfiConverterTreeIdINSTANCE.Read(reader),
		}
	default:
		panic(fmt.Sprintf("invalid enum value %v in FfiConverterIndex.Read()", id))
	}
}

func (FfiConverterIndex) Write(writer io.Writer, value Index) {
	switch variant_value := value.(type) {
	case IndexKey:
		writeInt32(writer, 1)
		FfiConverterStringINSTANCE.Write(writer, variant_value.Key)
	case IndexSeq:
		writeInt32(writer, 2)
		FfiConverterUint32INSTANCE.Write(writer, variant_value.Index)
	case IndexNode:
		writeInt32(writer, 3)
		FfiConverterTreeIdINSTANCE.Write(writer, variant_value.Target)
	default:
		_ = variant_value
		panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterIndex.Write", value))
	}
}

type FfiDestroyerIndex struct{}

func (_ FfiDestroyerIndex) Destroy(value Index) {
	value.Destroy()
}

type JsonPathError struct {
	err error
}

// Convience method to turn *JsonPathError into error
// Avoiding treating nil pointer as non nil error interface
func (err *JsonPathError) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err JsonPathError) Error() string {
	return fmt.Sprintf("JsonPathError: %s", err.err.Error())
}

func (err JsonPathError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrJsonPathErrorInvalidJsonPath = fmt.Errorf("JsonPathErrorInvalidJsonPath")
var ErrJsonPathErrorEvaluationError = fmt.Errorf("JsonPathErrorEvaluationError")

// Variant structs
type JsonPathErrorInvalidJsonPath struct {
	message string
}

func NewJsonPathErrorInvalidJsonPath() *JsonPathError {
	return &JsonPathError{err: &JsonPathErrorInvalidJsonPath{}}
}

func (e JsonPathErrorInvalidJsonPath) destroy() {
}

func (err JsonPathErrorInvalidJsonPath) Error() string {
	return fmt.Sprintf("InvalidJsonPath: %s", err.message)
}

func (self JsonPathErrorInvalidJsonPath) Is(target error) bool {
	return target == ErrJsonPathErrorInvalidJsonPath
}

type JsonPathErrorEvaluationError struct {
	message string
}

func NewJsonPathErrorEvaluationError() *JsonPathError {
	return &JsonPathError{err: &JsonPathErrorEvaluationError{}}
}

func (e JsonPathErrorEvaluationError) destroy() {
}

func (err JsonPathErrorEvaluationError) Error() string {
	return fmt.Sprintf("EvaluationError: %s", err.message)
}

func (self JsonPathErrorEvaluationError) Is(target error) bool {
	return target == ErrJsonPathErrorEvaluationError
}

type FfiConverterJsonPathError struct{}

var FfiConverterJsonPathErrorINSTANCE = FfiConverterJsonPathError{}

func (c FfiConverterJsonPathError) Lift(eb RustBufferI) *JsonPathError {
	return LiftFromRustBuffer[*JsonPathError](c, eb)
}

func (c FfiConverterJsonPathError) Lower(value *JsonPathError) C.RustBuffer {
	return LowerIntoRustBuffer[*JsonPathError](c, value)
}

func (c FfiConverterJsonPathError) Read(reader io.Reader) *JsonPathError {
	errorID := readUint32(reader)

	message := FfiConverterStringINSTANCE.Read(reader)
	switch errorID {
	case 1:
		return &JsonPathError{&JsonPathErrorInvalidJsonPath{message}}
	case 2:
		return &JsonPathError{&JsonPathErrorEvaluationError{message}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterJsonPathError.Read()", errorID))
	}

}

func (c FfiConverterJsonPathError) Write(writer io.Writer, value *JsonPathError) {
	switch variantValue := value.err.(type) {
	case *JsonPathErrorInvalidJsonPath:
		writeInt32(writer, 1)
	case *JsonPathErrorEvaluationError:
		writeInt32(writer, 2)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterJsonPathError.Write", value))
	}
}

type FfiDestroyerJsonPathError struct{}

func (_ FfiDestroyerJsonPathError) Destroy(value *JsonPathError) {
	switch variantValue := value.err.(type) {
	case JsonPathErrorInvalidJsonPath:
		variantValue.destroy()
	case JsonPathErrorEvaluationError:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerJsonPathError.Destroy", value))
	}
}

type ListDiffItem interface {
	Destroy()
}

// Insert a new element into the list.
type ListDiffItemInsert struct {
	Insert []*ValueOrContainer
	IsMove bool
}

func (e ListDiffItemInsert) Destroy() {
	FfiDestroyerSequenceValueOrContainer{}.Destroy(e.Insert)
	FfiDestroyerBool{}.Destroy(e.IsMove)
}

// Delete n elements from the list at the current index.
type ListDiffItemDelete struct {
	Delete uint32
}

func (e ListDiffItemDelete) Destroy() {
	FfiDestroyerUint32{}.Destroy(e.Delete)
}

// Retain n elements in the list.
//
// This is used to keep the current index unchanged.
type ListDiffItemRetain struct {
	Retain uint32
}

func (e ListDiffItemRetain) Destroy() {
	FfiDestroyerUint32{}.Destroy(e.Retain)
}

type FfiConverterListDiffItem struct{}

var FfiConverterListDiffItemINSTANCE = FfiConverterListDiffItem{}

func (c FfiConverterListDiffItem) Lift(rb RustBufferI) ListDiffItem {
	return LiftFromRustBuffer[ListDiffItem](c, rb)
}

func (c FfiConverterListDiffItem) Lower(value ListDiffItem) C.RustBuffer {
	return LowerIntoRustBuffer[ListDiffItem](c, value)
}
func (FfiConverterListDiffItem) Read(reader io.Reader) ListDiffItem {
	id := readInt32(reader)
	switch id {
	case 1:
		return ListDiffItemInsert{
			FfiConverterSequenceValueOrContainerINSTANCE.Read(reader),
			FfiConverterBoolINSTANCE.Read(reader),
		}
	case 2:
		return ListDiffItemDelete{
			FfiConverterUint32INSTANCE.Read(reader),
		}
	case 3:
		return ListDiffItemRetain{
			FfiConverterUint32INSTANCE.Read(reader),
		}
	default:
		panic(fmt.Sprintf("invalid enum value %v in FfiConverterListDiffItem.Read()", id))
	}
}

func (FfiConverterListDiffItem) Write(writer io.Writer, value ListDiffItem) {
	switch variant_value := value.(type) {
	case ListDiffItemInsert:
		writeInt32(writer, 1)
		FfiConverterSequenceValueOrContainerINSTANCE.Write(writer, variant_value.Insert)
		FfiConverterBoolINSTANCE.Write(writer, variant_value.IsMove)
	case ListDiffItemDelete:
		writeInt32(writer, 2)
		FfiConverterUint32INSTANCE.Write(writer, variant_value.Delete)
	case ListDiffItemRetain:
		writeInt32(writer, 3)
		FfiConverterUint32INSTANCE.Write(writer, variant_value.Retain)
	default:
		_ = variant_value
		panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterListDiffItem.Write", value))
	}
}

type FfiDestroyerListDiffItem struct{}

func (_ FfiDestroyerListDiffItem) Destroy(value ListDiffItem) {
	value.Destroy()
}

type LoroEncodeError struct {
	err error
}

// Convience method to turn *LoroEncodeError into error
// Avoiding treating nil pointer as non nil error interface
func (err *LoroEncodeError) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err LoroEncodeError) Error() string {
	return fmt.Sprintf("LoroEncodeError: %s", err.err.Error())
}

func (err LoroEncodeError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrLoroEncodeErrorFrontiersNotFound = fmt.Errorf("LoroEncodeErrorFrontiersNotFound")
var ErrLoroEncodeErrorShallowSnapshotIncompatibleWithOldFormat = fmt.Errorf("LoroEncodeErrorShallowSnapshotIncompatibleWithOldFormat")
var ErrLoroEncodeErrorUnknownContainer = fmt.Errorf("LoroEncodeErrorUnknownContainer")

// Variant structs
type LoroEncodeErrorFrontiersNotFound struct {
	message string
}

func NewLoroEncodeErrorFrontiersNotFound() *LoroEncodeError {
	return &LoroEncodeError{err: &LoroEncodeErrorFrontiersNotFound{}}
}

func (e LoroEncodeErrorFrontiersNotFound) destroy() {
}

func (err LoroEncodeErrorFrontiersNotFound) Error() string {
	return fmt.Sprintf("FrontiersNotFound: %s", err.message)
}

func (self LoroEncodeErrorFrontiersNotFound) Is(target error) bool {
	return target == ErrLoroEncodeErrorFrontiersNotFound
}

type LoroEncodeErrorShallowSnapshotIncompatibleWithOldFormat struct {
	message string
}

func NewLoroEncodeErrorShallowSnapshotIncompatibleWithOldFormat() *LoroEncodeError {
	return &LoroEncodeError{err: &LoroEncodeErrorShallowSnapshotIncompatibleWithOldFormat{}}
}

func (e LoroEncodeErrorShallowSnapshotIncompatibleWithOldFormat) destroy() {
}

func (err LoroEncodeErrorShallowSnapshotIncompatibleWithOldFormat) Error() string {
	return fmt.Sprintf("ShallowSnapshotIncompatibleWithOldFormat: %s", err.message)
}

func (self LoroEncodeErrorShallowSnapshotIncompatibleWithOldFormat) Is(target error) bool {
	return target == ErrLoroEncodeErrorShallowSnapshotIncompatibleWithOldFormat
}

type LoroEncodeErrorUnknownContainer struct {
	message string
}

func NewLoroEncodeErrorUnknownContainer() *LoroEncodeError {
	return &LoroEncodeError{err: &LoroEncodeErrorUnknownContainer{}}
}

func (e LoroEncodeErrorUnknownContainer) destroy() {
}

func (err LoroEncodeErrorUnknownContainer) Error() string {
	return fmt.Sprintf("UnknownContainer: %s", err.message)
}

func (self LoroEncodeErrorUnknownContainer) Is(target error) bool {
	return target == ErrLoroEncodeErrorUnknownContainer
}

type FfiConverterLoroEncodeError struct{}

var FfiConverterLoroEncodeErrorINSTANCE = FfiConverterLoroEncodeError{}

func (c FfiConverterLoroEncodeError) Lift(eb RustBufferI) *LoroEncodeError {
	return LiftFromRustBuffer[*LoroEncodeError](c, eb)
}

func (c FfiConverterLoroEncodeError) Lower(value *LoroEncodeError) C.RustBuffer {
	return LowerIntoRustBuffer[*LoroEncodeError](c, value)
}

func (c FfiConverterLoroEncodeError) Read(reader io.Reader) *LoroEncodeError {
	errorID := readUint32(reader)

	message := FfiConverterStringINSTANCE.Read(reader)
	switch errorID {
	case 1:
		return &LoroEncodeError{&LoroEncodeErrorFrontiersNotFound{message}}
	case 2:
		return &LoroEncodeError{&LoroEncodeErrorShallowSnapshotIncompatibleWithOldFormat{message}}
	case 3:
		return &LoroEncodeError{&LoroEncodeErrorUnknownContainer{message}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterLoroEncodeError.Read()", errorID))
	}

}

func (c FfiConverterLoroEncodeError) Write(writer io.Writer, value *LoroEncodeError) {
	switch variantValue := value.err.(type) {
	case *LoroEncodeErrorFrontiersNotFound:
		writeInt32(writer, 1)
	case *LoroEncodeErrorShallowSnapshotIncompatibleWithOldFormat:
		writeInt32(writer, 2)
	case *LoroEncodeErrorUnknownContainer:
		writeInt32(writer, 3)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterLoroEncodeError.Write", value))
	}
}

type FfiDestroyerLoroEncodeError struct{}

func (_ FfiDestroyerLoroEncodeError) Destroy(value *LoroEncodeError) {
	switch variantValue := value.err.(type) {
	case LoroEncodeErrorFrontiersNotFound:
		variantValue.destroy()
	case LoroEncodeErrorShallowSnapshotIncompatibleWithOldFormat:
		variantValue.destroy()
	case LoroEncodeErrorUnknownContainer:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerLoroEncodeError.Destroy", value))
	}
}

type LoroError struct {
	err error
}

// Convience method to turn *LoroError into error
// Avoiding treating nil pointer as non nil error interface
func (err *LoroError) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err LoroError) Error() string {
	return fmt.Sprintf("LoroError: %s", err.err.Error())
}

func (err LoroError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrLoroErrorUnmatchedContext = fmt.Errorf("LoroErrorUnmatchedContext")
var ErrLoroErrorDecodeVersionVectorError = fmt.Errorf("LoroErrorDecodeVersionVectorError")
var ErrLoroErrorDecodeError = fmt.Errorf("LoroErrorDecodeError")
var ErrLoroErrorDecodeDataCorruptionError = fmt.Errorf("LoroErrorDecodeDataCorruptionError")
var ErrLoroErrorDecodeChecksumMismatchError = fmt.Errorf("LoroErrorDecodeChecksumMismatchError")
var ErrLoroErrorIncompatibleFutureEncodingError = fmt.Errorf("LoroErrorIncompatibleFutureEncodingError")
var ErrLoroErrorJsError = fmt.Errorf("LoroErrorJsError")
var ErrLoroErrorLockError = fmt.Errorf("LoroErrorLockError")
var ErrLoroErrorDuplicatedTransactionError = fmt.Errorf("LoroErrorDuplicatedTransactionError")
var ErrLoroErrorNotFoundError = fmt.Errorf("LoroErrorNotFoundError")
var ErrLoroErrorTransactionError = fmt.Errorf("LoroErrorTransactionError")
var ErrLoroErrorOutOfBound = fmt.Errorf("LoroErrorOutOfBound")
var ErrLoroErrorUsedOpId = fmt.Errorf("LoroErrorUsedOpId")
var ErrLoroErrorTreeError = fmt.Errorf("LoroErrorTreeError")
var ErrLoroErrorArgErr = fmt.Errorf("LoroErrorArgErr")
var ErrLoroErrorAutoCommitNotStarted = fmt.Errorf("LoroErrorAutoCommitNotStarted")
var ErrLoroErrorStyleConfigMissing = fmt.Errorf("LoroErrorStyleConfigMissing")
var ErrLoroErrorUnknown = fmt.Errorf("LoroErrorUnknown")
var ErrLoroErrorFrontiersNotFound = fmt.Errorf("LoroErrorFrontiersNotFound")
var ErrLoroErrorImportWhenInTxn = fmt.Errorf("LoroErrorImportWhenInTxn")
var ErrLoroErrorMisuseDetachedContainer = fmt.Errorf("LoroErrorMisuseDetachedContainer")
var ErrLoroErrorNotImplemented = fmt.Errorf("LoroErrorNotImplemented")
var ErrLoroErrorReattachAttachedContainer = fmt.Errorf("LoroErrorReattachAttachedContainer")
var ErrLoroErrorEditWhenDetached = fmt.Errorf("LoroErrorEditWhenDetached")
var ErrLoroErrorUndoInvalidIdSpan = fmt.Errorf("LoroErrorUndoInvalidIdSpan")
var ErrLoroErrorUndoWithDifferentPeerId = fmt.Errorf("LoroErrorUndoWithDifferentPeerId")
var ErrLoroErrorInvalidJsonSchema = fmt.Errorf("LoroErrorInvalidJsonSchema")
var ErrLoroErrorUtf8InUnicodeCodePoint = fmt.Errorf("LoroErrorUtf8InUnicodeCodePoint")
var ErrLoroErrorUtf16InUnicodeCodePoint = fmt.Errorf("LoroErrorUtf16InUnicodeCodePoint")
var ErrLoroErrorEndIndexLessThanStartIndex = fmt.Errorf("LoroErrorEndIndexLessThanStartIndex")
var ErrLoroErrorInvalidRootContainerName = fmt.Errorf("LoroErrorInvalidRootContainerName")
var ErrLoroErrorImportUpdatesThatDependsOnOutdatedVersion = fmt.Errorf("LoroErrorImportUpdatesThatDependsOnOutdatedVersion")
var ErrLoroErrorSwitchToVersionBeforeShallowRoot = fmt.Errorf("LoroErrorSwitchToVersionBeforeShallowRoot")
var ErrLoroErrorContainerDeleted = fmt.Errorf("LoroErrorContainerDeleted")
var ErrLoroErrorConcurrentOpsWithSamePeerId = fmt.Errorf("LoroErrorConcurrentOpsWithSamePeerId")
var ErrLoroErrorInvalidPeerId = fmt.Errorf("LoroErrorInvalidPeerId")
var ErrLoroErrorContainersNotFound = fmt.Errorf("LoroErrorContainersNotFound")
var ErrLoroErrorUndoGroupAlreadyStarted = fmt.Errorf("LoroErrorUndoGroupAlreadyStarted")

// Variant structs
type LoroErrorUnmatchedContext struct {
	message string
}

func NewLoroErrorUnmatchedContext() *LoroError {
	return &LoroError{err: &LoroErrorUnmatchedContext{}}
}

func (e LoroErrorUnmatchedContext) destroy() {
}

func (err LoroErrorUnmatchedContext) Error() string {
	return fmt.Sprintf("UnmatchedContext: %s", err.message)
}

func (self LoroErrorUnmatchedContext) Is(target error) bool {
	return target == ErrLoroErrorUnmatchedContext
}

type LoroErrorDecodeVersionVectorError struct {
	message string
}

func NewLoroErrorDecodeVersionVectorError() *LoroError {
	return &LoroError{err: &LoroErrorDecodeVersionVectorError{}}
}

func (e LoroErrorDecodeVersionVectorError) destroy() {
}

func (err LoroErrorDecodeVersionVectorError) Error() string {
	return fmt.Sprintf("DecodeVersionVectorError: %s", err.message)
}

func (self LoroErrorDecodeVersionVectorError) Is(target error) bool {
	return target == ErrLoroErrorDecodeVersionVectorError
}

type LoroErrorDecodeError struct {
	message string
}

func NewLoroErrorDecodeError() *LoroError {
	return &LoroError{err: &LoroErrorDecodeError{}}
}

func (e LoroErrorDecodeError) destroy() {
}

func (err LoroErrorDecodeError) Error() string {
	return fmt.Sprintf("DecodeError: %s", err.message)
}

func (self LoroErrorDecodeError) Is(target error) bool {
	return target == ErrLoroErrorDecodeError
}

type LoroErrorDecodeDataCorruptionError struct {
	message string
}

func NewLoroErrorDecodeDataCorruptionError() *LoroError {
	return &LoroError{err: &LoroErrorDecodeDataCorruptionError{}}
}

func (e LoroErrorDecodeDataCorruptionError) destroy() {
}

func (err LoroErrorDecodeDataCorruptionError) Error() string {
	return fmt.Sprintf("DecodeDataCorruptionError: %s", err.message)
}

func (self LoroErrorDecodeDataCorruptionError) Is(target error) bool {
	return target == ErrLoroErrorDecodeDataCorruptionError
}

type LoroErrorDecodeChecksumMismatchError struct {
	message string
}

func NewLoroErrorDecodeChecksumMismatchError() *LoroError {
	return &LoroError{err: &LoroErrorDecodeChecksumMismatchError{}}
}

func (e LoroErrorDecodeChecksumMismatchError) destroy() {
}

func (err LoroErrorDecodeChecksumMismatchError) Error() string {
	return fmt.Sprintf("DecodeChecksumMismatchError: %s", err.message)
}

func (self LoroErrorDecodeChecksumMismatchError) Is(target error) bool {
	return target == ErrLoroErrorDecodeChecksumMismatchError
}

type LoroErrorIncompatibleFutureEncodingError struct {
	message string
}

func NewLoroErrorIncompatibleFutureEncodingError() *LoroError {
	return &LoroError{err: &LoroErrorIncompatibleFutureEncodingError{}}
}

func (e LoroErrorIncompatibleFutureEncodingError) destroy() {
}

func (err LoroErrorIncompatibleFutureEncodingError) Error() string {
	return fmt.Sprintf("IncompatibleFutureEncodingError: %s", err.message)
}

func (self LoroErrorIncompatibleFutureEncodingError) Is(target error) bool {
	return target == ErrLoroErrorIncompatibleFutureEncodingError
}

type LoroErrorJsError struct {
	message string
}

func NewLoroErrorJsError() *LoroError {
	return &LoroError{err: &LoroErrorJsError{}}
}

func (e LoroErrorJsError) destroy() {
}

func (err LoroErrorJsError) Error() string {
	return fmt.Sprintf("JsError: %s", err.message)
}

func (self LoroErrorJsError) Is(target error) bool {
	return target == ErrLoroErrorJsError
}

type LoroErrorLockError struct {
	message string
}

func NewLoroErrorLockError() *LoroError {
	return &LoroError{err: &LoroErrorLockError{}}
}

func (e LoroErrorLockError) destroy() {
}

func (err LoroErrorLockError) Error() string {
	return fmt.Sprintf("LockError: %s", err.message)
}

func (self LoroErrorLockError) Is(target error) bool {
	return target == ErrLoroErrorLockError
}

type LoroErrorDuplicatedTransactionError struct {
	message string
}

func NewLoroErrorDuplicatedTransactionError() *LoroError {
	return &LoroError{err: &LoroErrorDuplicatedTransactionError{}}
}

func (e LoroErrorDuplicatedTransactionError) destroy() {
}

func (err LoroErrorDuplicatedTransactionError) Error() string {
	return fmt.Sprintf("DuplicatedTransactionError: %s", err.message)
}

func (self LoroErrorDuplicatedTransactionError) Is(target error) bool {
	return target == ErrLoroErrorDuplicatedTransactionError
}

type LoroErrorNotFoundError struct {
	message string
}

func NewLoroErrorNotFoundError() *LoroError {
	return &LoroError{err: &LoroErrorNotFoundError{}}
}

func (e LoroErrorNotFoundError) destroy() {
}

func (err LoroErrorNotFoundError) Error() string {
	return fmt.Sprintf("NotFoundError: %s", err.message)
}

func (self LoroErrorNotFoundError) Is(target error) bool {
	return target == ErrLoroErrorNotFoundError
}

type LoroErrorTransactionError struct {
	message string
}

func NewLoroErrorTransactionError() *LoroError {
	return &LoroError{err: &LoroErrorTransactionError{}}
}

func (e LoroErrorTransactionError) destroy() {
}

func (err LoroErrorTransactionError) Error() string {
	return fmt.Sprintf("TransactionError: %s", err.message)
}

func (self LoroErrorTransactionError) Is(target error) bool {
	return target == ErrLoroErrorTransactionError
}

type LoroErrorOutOfBound struct {
	message string
}

func NewLoroErrorOutOfBound() *LoroError {
	return &LoroError{err: &LoroErrorOutOfBound{}}
}

func (e LoroErrorOutOfBound) destroy() {
}

func (err LoroErrorOutOfBound) Error() string {
	return fmt.Sprintf("OutOfBound: %s", err.message)
}

func (self LoroErrorOutOfBound) Is(target error) bool {
	return target == ErrLoroErrorOutOfBound
}

type LoroErrorUsedOpId struct {
	message string
}

func NewLoroErrorUsedOpId() *LoroError {
	return &LoroError{err: &LoroErrorUsedOpId{}}
}

func (e LoroErrorUsedOpId) destroy() {
}

func (err LoroErrorUsedOpId) Error() string {
	return fmt.Sprintf("UsedOpId: %s", err.message)
}

func (self LoroErrorUsedOpId) Is(target error) bool {
	return target == ErrLoroErrorUsedOpId
}

type LoroErrorTreeError struct {
	message string
}

func NewLoroErrorTreeError() *LoroError {
	return &LoroError{err: &LoroErrorTreeError{}}
}

func (e LoroErrorTreeError) destroy() {
}

func (err LoroErrorTreeError) Error() string {
	return fmt.Sprintf("TreeError: %s", err.message)
}

func (self LoroErrorTreeError) Is(target error) bool {
	return target == ErrLoroErrorTreeError
}

type LoroErrorArgErr struct {
	message string
}

func NewLoroErrorArgErr() *LoroError {
	return &LoroError{err: &LoroErrorArgErr{}}
}

func (e LoroErrorArgErr) destroy() {
}

func (err LoroErrorArgErr) Error() string {
	return fmt.Sprintf("ArgErr: %s", err.message)
}

func (self LoroErrorArgErr) Is(target error) bool {
	return target == ErrLoroErrorArgErr
}

type LoroErrorAutoCommitNotStarted struct {
	message string
}

func NewLoroErrorAutoCommitNotStarted() *LoroError {
	return &LoroError{err: &LoroErrorAutoCommitNotStarted{}}
}

func (e LoroErrorAutoCommitNotStarted) destroy() {
}

func (err LoroErrorAutoCommitNotStarted) Error() string {
	return fmt.Sprintf("AutoCommitNotStarted: %s", err.message)
}

func (self LoroErrorAutoCommitNotStarted) Is(target error) bool {
	return target == ErrLoroErrorAutoCommitNotStarted
}

type LoroErrorStyleConfigMissing struct {
	message string
}

func NewLoroErrorStyleConfigMissing() *LoroError {
	return &LoroError{err: &LoroErrorStyleConfigMissing{}}
}

func (e LoroErrorStyleConfigMissing) destroy() {
}

func (err LoroErrorStyleConfigMissing) Error() string {
	return fmt.Sprintf("StyleConfigMissing: %s", err.message)
}

func (self LoroErrorStyleConfigMissing) Is(target error) bool {
	return target == ErrLoroErrorStyleConfigMissing
}

type LoroErrorUnknown struct {
	message string
}

func NewLoroErrorUnknown() *LoroError {
	return &LoroError{err: &LoroErrorUnknown{}}
}

func (e LoroErrorUnknown) destroy() {
}

func (err LoroErrorUnknown) Error() string {
	return fmt.Sprintf("Unknown: %s", err.message)
}

func (self LoroErrorUnknown) Is(target error) bool {
	return target == ErrLoroErrorUnknown
}

type LoroErrorFrontiersNotFound struct {
	message string
}

func NewLoroErrorFrontiersNotFound() *LoroError {
	return &LoroError{err: &LoroErrorFrontiersNotFound{}}
}

func (e LoroErrorFrontiersNotFound) destroy() {
}

func (err LoroErrorFrontiersNotFound) Error() string {
	return fmt.Sprintf("FrontiersNotFound: %s", err.message)
}

func (self LoroErrorFrontiersNotFound) Is(target error) bool {
	return target == ErrLoroErrorFrontiersNotFound
}

type LoroErrorImportWhenInTxn struct {
	message string
}

func NewLoroErrorImportWhenInTxn() *LoroError {
	return &LoroError{err: &LoroErrorImportWhenInTxn{}}
}

func (e LoroErrorImportWhenInTxn) destroy() {
}

func (err LoroErrorImportWhenInTxn) Error() string {
	return fmt.Sprintf("ImportWhenInTxn: %s", err.message)
}

func (self LoroErrorImportWhenInTxn) Is(target error) bool {
	return target == ErrLoroErrorImportWhenInTxn
}

type LoroErrorMisuseDetachedContainer struct {
	message string
}

func NewLoroErrorMisuseDetachedContainer() *LoroError {
	return &LoroError{err: &LoroErrorMisuseDetachedContainer{}}
}

func (e LoroErrorMisuseDetachedContainer) destroy() {
}

func (err LoroErrorMisuseDetachedContainer) Error() string {
	return fmt.Sprintf("MisuseDetachedContainer: %s", err.message)
}

func (self LoroErrorMisuseDetachedContainer) Is(target error) bool {
	return target == ErrLoroErrorMisuseDetachedContainer
}

type LoroErrorNotImplemented struct {
	message string
}

func NewLoroErrorNotImplemented() *LoroError {
	return &LoroError{err: &LoroErrorNotImplemented{}}
}

func (e LoroErrorNotImplemented) destroy() {
}

func (err LoroErrorNotImplemented) Error() string {
	return fmt.Sprintf("NotImplemented: %s", err.message)
}

func (self LoroErrorNotImplemented) Is(target error) bool {
	return target == ErrLoroErrorNotImplemented
}

type LoroErrorReattachAttachedContainer struct {
	message string
}

func NewLoroErrorReattachAttachedContainer() *LoroError {
	return &LoroError{err: &LoroErrorReattachAttachedContainer{}}
}

func (e LoroErrorReattachAttachedContainer) destroy() {
}

func (err LoroErrorReattachAttachedContainer) Error() string {
	return fmt.Sprintf("ReattachAttachedContainer: %s", err.message)
}

func (self LoroErrorReattachAttachedContainer) Is(target error) bool {
	return target == ErrLoroErrorReattachAttachedContainer
}

type LoroErrorEditWhenDetached struct {
	message string
}

func NewLoroErrorEditWhenDetached() *LoroError {
	return &LoroError{err: &LoroErrorEditWhenDetached{}}
}

func (e LoroErrorEditWhenDetached) destroy() {
}

func (err LoroErrorEditWhenDetached) Error() string {
	return fmt.Sprintf("EditWhenDetached: %s", err.message)
}

func (self LoroErrorEditWhenDetached) Is(target error) bool {
	return target == ErrLoroErrorEditWhenDetached
}

type LoroErrorUndoInvalidIdSpan struct {
	message string
}

func NewLoroErrorUndoInvalidIdSpan() *LoroError {
	return &LoroError{err: &LoroErrorUndoInvalidIdSpan{}}
}

func (e LoroErrorUndoInvalidIdSpan) destroy() {
}

func (err LoroErrorUndoInvalidIdSpan) Error() string {
	return fmt.Sprintf("UndoInvalidIdSpan: %s", err.message)
}

func (self LoroErrorUndoInvalidIdSpan) Is(target error) bool {
	return target == ErrLoroErrorUndoInvalidIdSpan
}

type LoroErrorUndoWithDifferentPeerId struct {
	message string
}

func NewLoroErrorUndoWithDifferentPeerId() *LoroError {
	return &LoroError{err: &LoroErrorUndoWithDifferentPeerId{}}
}

func (e LoroErrorUndoWithDifferentPeerId) destroy() {
}

func (err LoroErrorUndoWithDifferentPeerId) Error() string {
	return fmt.Sprintf("UndoWithDifferentPeerId: %s", err.message)
}

func (self LoroErrorUndoWithDifferentPeerId) Is(target error) bool {
	return target == ErrLoroErrorUndoWithDifferentPeerId
}

type LoroErrorInvalidJsonSchema struct {
	message string
}

func NewLoroErrorInvalidJsonSchema() *LoroError {
	return &LoroError{err: &LoroErrorInvalidJsonSchema{}}
}

func (e LoroErrorInvalidJsonSchema) destroy() {
}

func (err LoroErrorInvalidJsonSchema) Error() string {
	return fmt.Sprintf("InvalidJsonSchema: %s", err.message)
}

func (self LoroErrorInvalidJsonSchema) Is(target error) bool {
	return target == ErrLoroErrorInvalidJsonSchema
}

type LoroErrorUtf8InUnicodeCodePoint struct {
	message string
}

func NewLoroErrorUtf8InUnicodeCodePoint() *LoroError {
	return &LoroError{err: &LoroErrorUtf8InUnicodeCodePoint{}}
}

func (e LoroErrorUtf8InUnicodeCodePoint) destroy() {
}

func (err LoroErrorUtf8InUnicodeCodePoint) Error() string {
	return fmt.Sprintf("Utf8InUnicodeCodePoint: %s", err.message)
}

func (self LoroErrorUtf8InUnicodeCodePoint) Is(target error) bool {
	return target == ErrLoroErrorUtf8InUnicodeCodePoint
}

type LoroErrorUtf16InUnicodeCodePoint struct {
	message string
}

func NewLoroErrorUtf16InUnicodeCodePoint() *LoroError {
	return &LoroError{err: &LoroErrorUtf16InUnicodeCodePoint{}}
}

func (e LoroErrorUtf16InUnicodeCodePoint) destroy() {
}

func (err LoroErrorUtf16InUnicodeCodePoint) Error() string {
	return fmt.Sprintf("Utf16InUnicodeCodePoint: %s", err.message)
}

func (self LoroErrorUtf16InUnicodeCodePoint) Is(target error) bool {
	return target == ErrLoroErrorUtf16InUnicodeCodePoint
}

type LoroErrorEndIndexLessThanStartIndex struct {
	message string
}

func NewLoroErrorEndIndexLessThanStartIndex() *LoroError {
	return &LoroError{err: &LoroErrorEndIndexLessThanStartIndex{}}
}

func (e LoroErrorEndIndexLessThanStartIndex) destroy() {
}

func (err LoroErrorEndIndexLessThanStartIndex) Error() string {
	return fmt.Sprintf("EndIndexLessThanStartIndex: %s", err.message)
}

func (self LoroErrorEndIndexLessThanStartIndex) Is(target error) bool {
	return target == ErrLoroErrorEndIndexLessThanStartIndex
}

type LoroErrorInvalidRootContainerName struct {
	message string
}

func NewLoroErrorInvalidRootContainerName() *LoroError {
	return &LoroError{err: &LoroErrorInvalidRootContainerName{}}
}

func (e LoroErrorInvalidRootContainerName) destroy() {
}

func (err LoroErrorInvalidRootContainerName) Error() string {
	return fmt.Sprintf("InvalidRootContainerName: %s", err.message)
}

func (self LoroErrorInvalidRootContainerName) Is(target error) bool {
	return target == ErrLoroErrorInvalidRootContainerName
}

type LoroErrorImportUpdatesThatDependsOnOutdatedVersion struct {
	message string
}

func NewLoroErrorImportUpdatesThatDependsOnOutdatedVersion() *LoroError {
	return &LoroError{err: &LoroErrorImportUpdatesThatDependsOnOutdatedVersion{}}
}

func (e LoroErrorImportUpdatesThatDependsOnOutdatedVersion) destroy() {
}

func (err LoroErrorImportUpdatesThatDependsOnOutdatedVersion) Error() string {
	return fmt.Sprintf("ImportUpdatesThatDependsOnOutdatedVersion: %s", err.message)
}

func (self LoroErrorImportUpdatesThatDependsOnOutdatedVersion) Is(target error) bool {
	return target == ErrLoroErrorImportUpdatesThatDependsOnOutdatedVersion
}

type LoroErrorSwitchToVersionBeforeShallowRoot struct {
	message string
}

func NewLoroErrorSwitchToVersionBeforeShallowRoot() *LoroError {
	return &LoroError{err: &LoroErrorSwitchToVersionBeforeShallowRoot{}}
}

func (e LoroErrorSwitchToVersionBeforeShallowRoot) destroy() {
}

func (err LoroErrorSwitchToVersionBeforeShallowRoot) Error() string {
	return fmt.Sprintf("SwitchToVersionBeforeShallowRoot: %s", err.message)
}

func (self LoroErrorSwitchToVersionBeforeShallowRoot) Is(target error) bool {
	return target == ErrLoroErrorSwitchToVersionBeforeShallowRoot
}

type LoroErrorContainerDeleted struct {
	message string
}

func NewLoroErrorContainerDeleted() *LoroError {
	return &LoroError{err: &LoroErrorContainerDeleted{}}
}

func (e LoroErrorContainerDeleted) destroy() {
}

func (err LoroErrorContainerDeleted) Error() string {
	return fmt.Sprintf("ContainerDeleted: %s", err.message)
}

func (self LoroErrorContainerDeleted) Is(target error) bool {
	return target == ErrLoroErrorContainerDeleted
}

type LoroErrorConcurrentOpsWithSamePeerId struct {
	message string
}

func NewLoroErrorConcurrentOpsWithSamePeerId() *LoroError {
	return &LoroError{err: &LoroErrorConcurrentOpsWithSamePeerId{}}
}

func (e LoroErrorConcurrentOpsWithSamePeerId) destroy() {
}

func (err LoroErrorConcurrentOpsWithSamePeerId) Error() string {
	return fmt.Sprintf("ConcurrentOpsWithSamePeerId: %s", err.message)
}

func (self LoroErrorConcurrentOpsWithSamePeerId) Is(target error) bool {
	return target == ErrLoroErrorConcurrentOpsWithSamePeerId
}

type LoroErrorInvalidPeerId struct {
	message string
}

func NewLoroErrorInvalidPeerId() *LoroError {
	return &LoroError{err: &LoroErrorInvalidPeerId{}}
}

func (e LoroErrorInvalidPeerId) destroy() {
}

func (err LoroErrorInvalidPeerId) Error() string {
	return fmt.Sprintf("InvalidPeerId: %s", err.message)
}

func (self LoroErrorInvalidPeerId) Is(target error) bool {
	return target == ErrLoroErrorInvalidPeerId
}

type LoroErrorContainersNotFound struct {
	message string
}

func NewLoroErrorContainersNotFound() *LoroError {
	return &LoroError{err: &LoroErrorContainersNotFound{}}
}

func (e LoroErrorContainersNotFound) destroy() {
}

func (err LoroErrorContainersNotFound) Error() string {
	return fmt.Sprintf("ContainersNotFound: %s", err.message)
}

func (self LoroErrorContainersNotFound) Is(target error) bool {
	return target == ErrLoroErrorContainersNotFound
}

type LoroErrorUndoGroupAlreadyStarted struct {
	message string
}

func NewLoroErrorUndoGroupAlreadyStarted() *LoroError {
	return &LoroError{err: &LoroErrorUndoGroupAlreadyStarted{}}
}

func (e LoroErrorUndoGroupAlreadyStarted) destroy() {
}

func (err LoroErrorUndoGroupAlreadyStarted) Error() string {
	return fmt.Sprintf("UndoGroupAlreadyStarted: %s", err.message)
}

func (self LoroErrorUndoGroupAlreadyStarted) Is(target error) bool {
	return target == ErrLoroErrorUndoGroupAlreadyStarted
}

type FfiConverterLoroError struct{}

var FfiConverterLoroErrorINSTANCE = FfiConverterLoroError{}

func (c FfiConverterLoroError) Lift(eb RustBufferI) *LoroError {
	return LiftFromRustBuffer[*LoroError](c, eb)
}

func (c FfiConverterLoroError) Lower(value *LoroError) C.RustBuffer {
	return LowerIntoRustBuffer[*LoroError](c, value)
}

func (c FfiConverterLoroError) Read(reader io.Reader) *LoroError {
	errorID := readUint32(reader)

	message := FfiConverterStringINSTANCE.Read(reader)
	switch errorID {
	case 1:
		return &LoroError{&LoroErrorUnmatchedContext{message}}
	case 2:
		return &LoroError{&LoroErrorDecodeVersionVectorError{message}}
	case 3:
		return &LoroError{&LoroErrorDecodeError{message}}
	case 4:
		return &LoroError{&LoroErrorDecodeDataCorruptionError{message}}
	case 5:
		return &LoroError{&LoroErrorDecodeChecksumMismatchError{message}}
	case 6:
		return &LoroError{&LoroErrorIncompatibleFutureEncodingError{message}}
	case 7:
		return &LoroError{&LoroErrorJsError{message}}
	case 8:
		return &LoroError{&LoroErrorLockError{message}}
	case 9:
		return &LoroError{&LoroErrorDuplicatedTransactionError{message}}
	case 10:
		return &LoroError{&LoroErrorNotFoundError{message}}
	case 11:
		return &LoroError{&LoroErrorTransactionError{message}}
	case 12:
		return &LoroError{&LoroErrorOutOfBound{message}}
	case 13:
		return &LoroError{&LoroErrorUsedOpId{message}}
	case 14:
		return &LoroError{&LoroErrorTreeError{message}}
	case 15:
		return &LoroError{&LoroErrorArgErr{message}}
	case 16:
		return &LoroError{&LoroErrorAutoCommitNotStarted{message}}
	case 17:
		return &LoroError{&LoroErrorStyleConfigMissing{message}}
	case 18:
		return &LoroError{&LoroErrorUnknown{message}}
	case 19:
		return &LoroError{&LoroErrorFrontiersNotFound{message}}
	case 20:
		return &LoroError{&LoroErrorImportWhenInTxn{message}}
	case 21:
		return &LoroError{&LoroErrorMisuseDetachedContainer{message}}
	case 22:
		return &LoroError{&LoroErrorNotImplemented{message}}
	case 23:
		return &LoroError{&LoroErrorReattachAttachedContainer{message}}
	case 24:
		return &LoroError{&LoroErrorEditWhenDetached{message}}
	case 25:
		return &LoroError{&LoroErrorUndoInvalidIdSpan{message}}
	case 26:
		return &LoroError{&LoroErrorUndoWithDifferentPeerId{message}}
	case 27:
		return &LoroError{&LoroErrorInvalidJsonSchema{message}}
	case 28:
		return &LoroError{&LoroErrorUtf8InUnicodeCodePoint{message}}
	case 29:
		return &LoroError{&LoroErrorUtf16InUnicodeCodePoint{message}}
	case 30:
		return &LoroError{&LoroErrorEndIndexLessThanStartIndex{message}}
	case 31:
		return &LoroError{&LoroErrorInvalidRootContainerName{message}}
	case 32:
		return &LoroError{&LoroErrorImportUpdatesThatDependsOnOutdatedVersion{message}}
	case 33:
		return &LoroError{&LoroErrorSwitchToVersionBeforeShallowRoot{message}}
	case 34:
		return &LoroError{&LoroErrorContainerDeleted{message}}
	case 35:
		return &LoroError{&LoroErrorConcurrentOpsWithSamePeerId{message}}
	case 36:
		return &LoroError{&LoroErrorInvalidPeerId{message}}
	case 37:
		return &LoroError{&LoroErrorContainersNotFound{message}}
	case 38:
		return &LoroError{&LoroErrorUndoGroupAlreadyStarted{message}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterLoroError.Read()", errorID))
	}

}

func (c FfiConverterLoroError) Write(writer io.Writer, value *LoroError) {
	switch variantValue := value.err.(type) {
	case *LoroErrorUnmatchedContext:
		writeInt32(writer, 1)
	case *LoroErrorDecodeVersionVectorError:
		writeInt32(writer, 2)
	case *LoroErrorDecodeError:
		writeInt32(writer, 3)
	case *LoroErrorDecodeDataCorruptionError:
		writeInt32(writer, 4)
	case *LoroErrorDecodeChecksumMismatchError:
		writeInt32(writer, 5)
	case *LoroErrorIncompatibleFutureEncodingError:
		writeInt32(writer, 6)
	case *LoroErrorJsError:
		writeInt32(writer, 7)
	case *LoroErrorLockError:
		writeInt32(writer, 8)
	case *LoroErrorDuplicatedTransactionError:
		writeInt32(writer, 9)
	case *LoroErrorNotFoundError:
		writeInt32(writer, 10)
	case *LoroErrorTransactionError:
		writeInt32(writer, 11)
	case *LoroErrorOutOfBound:
		writeInt32(writer, 12)
	case *LoroErrorUsedOpId:
		writeInt32(writer, 13)
	case *LoroErrorTreeError:
		writeInt32(writer, 14)
	case *LoroErrorArgErr:
		writeInt32(writer, 15)
	case *LoroErrorAutoCommitNotStarted:
		writeInt32(writer, 16)
	case *LoroErrorStyleConfigMissing:
		writeInt32(writer, 17)
	case *LoroErrorUnknown:
		writeInt32(writer, 18)
	case *LoroErrorFrontiersNotFound:
		writeInt32(writer, 19)
	case *LoroErrorImportWhenInTxn:
		writeInt32(writer, 20)
	case *LoroErrorMisuseDetachedContainer:
		writeInt32(writer, 21)
	case *LoroErrorNotImplemented:
		writeInt32(writer, 22)
	case *LoroErrorReattachAttachedContainer:
		writeInt32(writer, 23)
	case *LoroErrorEditWhenDetached:
		writeInt32(writer, 24)
	case *LoroErrorUndoInvalidIdSpan:
		writeInt32(writer, 25)
	case *LoroErrorUndoWithDifferentPeerId:
		writeInt32(writer, 26)
	case *LoroErrorInvalidJsonSchema:
		writeInt32(writer, 27)
	case *LoroErrorUtf8InUnicodeCodePoint:
		writeInt32(writer, 28)
	case *LoroErrorUtf16InUnicodeCodePoint:
		writeInt32(writer, 29)
	case *LoroErrorEndIndexLessThanStartIndex:
		writeInt32(writer, 30)
	case *LoroErrorInvalidRootContainerName:
		writeInt32(writer, 31)
	case *LoroErrorImportUpdatesThatDependsOnOutdatedVersion:
		writeInt32(writer, 32)
	case *LoroErrorSwitchToVersionBeforeShallowRoot:
		writeInt32(writer, 33)
	case *LoroErrorContainerDeleted:
		writeInt32(writer, 34)
	case *LoroErrorConcurrentOpsWithSamePeerId:
		writeInt32(writer, 35)
	case *LoroErrorInvalidPeerId:
		writeInt32(writer, 36)
	case *LoroErrorContainersNotFound:
		writeInt32(writer, 37)
	case *LoroErrorUndoGroupAlreadyStarted:
		writeInt32(writer, 38)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterLoroError.Write", value))
	}
}

type FfiDestroyerLoroError struct{}

func (_ FfiDestroyerLoroError) Destroy(value *LoroError) {
	switch variantValue := value.err.(type) {
	case LoroErrorUnmatchedContext:
		variantValue.destroy()
	case LoroErrorDecodeVersionVectorError:
		variantValue.destroy()
	case LoroErrorDecodeError:
		variantValue.destroy()
	case LoroErrorDecodeDataCorruptionError:
		variantValue.destroy()
	case LoroErrorDecodeChecksumMismatchError:
		variantValue.destroy()
	case LoroErrorIncompatibleFutureEncodingError:
		variantValue.destroy()
	case LoroErrorJsError:
		variantValue.destroy()
	case LoroErrorLockError:
		variantValue.destroy()
	case LoroErrorDuplicatedTransactionError:
		variantValue.destroy()
	case LoroErrorNotFoundError:
		variantValue.destroy()
	case LoroErrorTransactionError:
		variantValue.destroy()
	case LoroErrorOutOfBound:
		variantValue.destroy()
	case LoroErrorUsedOpId:
		variantValue.destroy()
	case LoroErrorTreeError:
		variantValue.destroy()
	case LoroErrorArgErr:
		variantValue.destroy()
	case LoroErrorAutoCommitNotStarted:
		variantValue.destroy()
	case LoroErrorStyleConfigMissing:
		variantValue.destroy()
	case LoroErrorUnknown:
		variantValue.destroy()
	case LoroErrorFrontiersNotFound:
		variantValue.destroy()
	case LoroErrorImportWhenInTxn:
		variantValue.destroy()
	case LoroErrorMisuseDetachedContainer:
		variantValue.destroy()
	case LoroErrorNotImplemented:
		variantValue.destroy()
	case LoroErrorReattachAttachedContainer:
		variantValue.destroy()
	case LoroErrorEditWhenDetached:
		variantValue.destroy()
	case LoroErrorUndoInvalidIdSpan:
		variantValue.destroy()
	case LoroErrorUndoWithDifferentPeerId:
		variantValue.destroy()
	case LoroErrorInvalidJsonSchema:
		variantValue.destroy()
	case LoroErrorUtf8InUnicodeCodePoint:
		variantValue.destroy()
	case LoroErrorUtf16InUnicodeCodePoint:
		variantValue.destroy()
	case LoroErrorEndIndexLessThanStartIndex:
		variantValue.destroy()
	case LoroErrorInvalidRootContainerName:
		variantValue.destroy()
	case LoroErrorImportUpdatesThatDependsOnOutdatedVersion:
		variantValue.destroy()
	case LoroErrorSwitchToVersionBeforeShallowRoot:
		variantValue.destroy()
	case LoroErrorContainerDeleted:
		variantValue.destroy()
	case LoroErrorConcurrentOpsWithSamePeerId:
		variantValue.destroy()
	case LoroErrorInvalidPeerId:
		variantValue.destroy()
	case LoroErrorContainersNotFound:
		variantValue.destroy()
	case LoroErrorUndoGroupAlreadyStarted:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerLoroError.Destroy", value))
	}
}

type LoroValue interface {
	Destroy()
}
type LoroValueNull struct {
}

func (e LoroValueNull) Destroy() {
}

type LoroValueBool struct {
	Value bool
}

func (e LoroValueBool) Destroy() {
	FfiDestroyerBool{}.Destroy(e.Value)
}

type LoroValueDouble struct {
	Value float64
}

func (e LoroValueDouble) Destroy() {
	FfiDestroyerFloat64{}.Destroy(e.Value)
}

type LoroValueI64 struct {
	Value int64
}

func (e LoroValueI64) Destroy() {
	FfiDestroyerInt64{}.Destroy(e.Value)
}

type LoroValueBinary struct {
	Value []byte
}

func (e LoroValueBinary) Destroy() {
	FfiDestroyerBytes{}.Destroy(e.Value)
}

type LoroValueString struct {
	Value string
}

func (e LoroValueString) Destroy() {
	FfiDestroyerString{}.Destroy(e.Value)
}

type LoroValueList struct {
	Value []LoroValue
}

func (e LoroValueList) Destroy() {
	FfiDestroyerSequenceLoroValue{}.Destroy(e.Value)
}

type LoroValueMap struct {
	Value map[string]LoroValue
}

func (e LoroValueMap) Destroy() {
	FfiDestroyerMapStringLoroValue{}.Destroy(e.Value)
}

type LoroValueContainer struct {
	Value ContainerId
}

func (e LoroValueContainer) Destroy() {
	FfiDestroyerContainerId{}.Destroy(e.Value)
}

type FfiConverterLoroValue struct{}

var FfiConverterLoroValueINSTANCE = FfiConverterLoroValue{}

func (c FfiConverterLoroValue) Lift(rb RustBufferI) LoroValue {
	return LiftFromRustBuffer[LoroValue](c, rb)
}

func (c FfiConverterLoroValue) Lower(value LoroValue) C.RustBuffer {
	return LowerIntoRustBuffer[LoroValue](c, value)
}
func (FfiConverterLoroValue) Read(reader io.Reader) LoroValue {
	id := readInt32(reader)
	switch id {
	case 1:
		return LoroValueNull{}
	case 2:
		return LoroValueBool{
			FfiConverterBoolINSTANCE.Read(reader),
		}
	case 3:
		return LoroValueDouble{
			FfiConverterFloat64INSTANCE.Read(reader),
		}
	case 4:
		return LoroValueI64{
			FfiConverterInt64INSTANCE.Read(reader),
		}
	case 5:
		return LoroValueBinary{
			FfiConverterBytesINSTANCE.Read(reader),
		}
	case 6:
		return LoroValueString{
			FfiConverterStringINSTANCE.Read(reader),
		}
	case 7:
		return LoroValueList{
			FfiConverterSequenceLoroValueINSTANCE.Read(reader),
		}
	case 8:
		return LoroValueMap{
			FfiConverterMapStringLoroValueINSTANCE.Read(reader),
		}
	case 9:
		return LoroValueContainer{
			FfiConverterContainerIdINSTANCE.Read(reader),
		}
	default:
		panic(fmt.Sprintf("invalid enum value %v in FfiConverterLoroValue.Read()", id))
	}
}

func (FfiConverterLoroValue) Write(writer io.Writer, value LoroValue) {
	switch variant_value := value.(type) {
	case LoroValueNull:
		writeInt32(writer, 1)
	case LoroValueBool:
		writeInt32(writer, 2)
		FfiConverterBoolINSTANCE.Write(writer, variant_value.Value)
	case LoroValueDouble:
		writeInt32(writer, 3)
		FfiConverterFloat64INSTANCE.Write(writer, variant_value.Value)
	case LoroValueI64:
		writeInt32(writer, 4)
		FfiConverterInt64INSTANCE.Write(writer, variant_value.Value)
	case LoroValueBinary:
		writeInt32(writer, 5)
		FfiConverterBytesINSTANCE.Write(writer, variant_value.Value)
	case LoroValueString:
		writeInt32(writer, 6)
		FfiConverterStringINSTANCE.Write(writer, variant_value.Value)
	case LoroValueList:
		writeInt32(writer, 7)
		FfiConverterSequenceLoroValueINSTANCE.Write(writer, variant_value.Value)
	case LoroValueMap:
		writeInt32(writer, 8)
		FfiConverterMapStringLoroValueINSTANCE.Write(writer, variant_value.Value)
	case LoroValueContainer:
		writeInt32(writer, 9)
		FfiConverterContainerIdINSTANCE.Write(writer, variant_value.Value)
	default:
		_ = variant_value
		panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterLoroValue.Write", value))
	}
}

type FfiDestroyerLoroValue struct{}

func (_ FfiDestroyerLoroValue) Destroy(value LoroValue) {
	value.Destroy()
}

type Ordering uint

const (
	OrderingLess    Ordering = 1
	OrderingEqual   Ordering = 2
	OrderingGreater Ordering = 3
)

type FfiConverterOrdering struct{}

var FfiConverterOrderingINSTANCE = FfiConverterOrdering{}

func (c FfiConverterOrdering) Lift(rb RustBufferI) Ordering {
	return LiftFromRustBuffer[Ordering](c, rb)
}

func (c FfiConverterOrdering) Lower(value Ordering) C.RustBuffer {
	return LowerIntoRustBuffer[Ordering](c, value)
}
func (FfiConverterOrdering) Read(reader io.Reader) Ordering {
	id := readInt32(reader)
	return Ordering(id)
}

func (FfiConverterOrdering) Write(writer io.Writer, value Ordering) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerOrdering struct{}

func (_ FfiDestroyerOrdering) Destroy(value Ordering) {
}

type Side uint

const (
	SideLeft   Side = 1
	SideMiddle Side = 2
	SideRight  Side = 3
)

type FfiConverterSide struct{}

var FfiConverterSideINSTANCE = FfiConverterSide{}

func (c FfiConverterSide) Lift(rb RustBufferI) Side {
	return LiftFromRustBuffer[Side](c, rb)
}

func (c FfiConverterSide) Lower(value Side) C.RustBuffer {
	return LowerIntoRustBuffer[Side](c, value)
}
func (FfiConverterSide) Read(reader io.Reader) Side {
	id := readInt32(reader)
	return Side(id)
}

func (FfiConverterSide) Write(writer io.Writer, value Side) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerSide struct{}

func (_ FfiDestroyerSide) Destroy(value Side) {
}

type TextDelta interface {
	Destroy()
}
type TextDeltaRetain struct {
	Retain     uint32
	Attributes *map[string]LoroValue
}

func (e TextDeltaRetain) Destroy() {
	FfiDestroyerUint32{}.Destroy(e.Retain)
	FfiDestroyerOptionalMapStringLoroValue{}.Destroy(e.Attributes)
}

type TextDeltaInsert struct {
	Insert     string
	Attributes *map[string]LoroValue
}

func (e TextDeltaInsert) Destroy() {
	FfiDestroyerString{}.Destroy(e.Insert)
	FfiDestroyerOptionalMapStringLoroValue{}.Destroy(e.Attributes)
}

type TextDeltaDelete struct {
	Delete uint32
}

func (e TextDeltaDelete) Destroy() {
	FfiDestroyerUint32{}.Destroy(e.Delete)
}

type FfiConverterTextDelta struct{}

var FfiConverterTextDeltaINSTANCE = FfiConverterTextDelta{}

func (c FfiConverterTextDelta) Lift(rb RustBufferI) TextDelta {
	return LiftFromRustBuffer[TextDelta](c, rb)
}

func (c FfiConverterTextDelta) Lower(value TextDelta) C.RustBuffer {
	return LowerIntoRustBuffer[TextDelta](c, value)
}
func (FfiConverterTextDelta) Read(reader io.Reader) TextDelta {
	id := readInt32(reader)
	switch id {
	case 1:
		return TextDeltaRetain{
			FfiConverterUint32INSTANCE.Read(reader),
			FfiConverterOptionalMapStringLoroValueINSTANCE.Read(reader),
		}
	case 2:
		return TextDeltaInsert{
			FfiConverterStringINSTANCE.Read(reader),
			FfiConverterOptionalMapStringLoroValueINSTANCE.Read(reader),
		}
	case 3:
		return TextDeltaDelete{
			FfiConverterUint32INSTANCE.Read(reader),
		}
	default:
		panic(fmt.Sprintf("invalid enum value %v in FfiConverterTextDelta.Read()", id))
	}
}

func (FfiConverterTextDelta) Write(writer io.Writer, value TextDelta) {
	switch variant_value := value.(type) {
	case TextDeltaRetain:
		writeInt32(writer, 1)
		FfiConverterUint32INSTANCE.Write(writer, variant_value.Retain)
		FfiConverterOptionalMapStringLoroValueINSTANCE.Write(writer, variant_value.Attributes)
	case TextDeltaInsert:
		writeInt32(writer, 2)
		FfiConverterStringINSTANCE.Write(writer, variant_value.Insert)
		FfiConverterOptionalMapStringLoroValueINSTANCE.Write(writer, variant_value.Attributes)
	case TextDeltaDelete:
		writeInt32(writer, 3)
		FfiConverterUint32INSTANCE.Write(writer, variant_value.Delete)
	default:
		_ = variant_value
		panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterTextDelta.Write", value))
	}
}

type FfiDestroyerTextDelta struct{}

func (_ FfiDestroyerTextDelta) Destroy(value TextDelta) {
	value.Destroy()
}

type TreeExternalDiff interface {
	Destroy()
}
type TreeExternalDiffCreate struct {
	Parent          TreeParentId
	Index           uint32
	FractionalIndex string
}

func (e TreeExternalDiffCreate) Destroy() {
	FfiDestroyerTreeParentId{}.Destroy(e.Parent)
	FfiDestroyerUint32{}.Destroy(e.Index)
	FfiDestroyerString{}.Destroy(e.FractionalIndex)
}

type TreeExternalDiffMove struct {
	Parent          TreeParentId
	Index           uint32
	FractionalIndex string
	OldParent       TreeParentId
	OldIndex        uint32
}

func (e TreeExternalDiffMove) Destroy() {
	FfiDestroyerTreeParentId{}.Destroy(e.Parent)
	FfiDestroyerUint32{}.Destroy(e.Index)
	FfiDestroyerString{}.Destroy(e.FractionalIndex)
	FfiDestroyerTreeParentId{}.Destroy(e.OldParent)
	FfiDestroyerUint32{}.Destroy(e.OldIndex)
}

type TreeExternalDiffDelete struct {
	OldParent TreeParentId
	OldIndex  uint32
}

func (e TreeExternalDiffDelete) Destroy() {
	FfiDestroyerTreeParentId{}.Destroy(e.OldParent)
	FfiDestroyerUint32{}.Destroy(e.OldIndex)
}

type FfiConverterTreeExternalDiff struct{}

var FfiConverterTreeExternalDiffINSTANCE = FfiConverterTreeExternalDiff{}

func (c FfiConverterTreeExternalDiff) Lift(rb RustBufferI) TreeExternalDiff {
	return LiftFromRustBuffer[TreeExternalDiff](c, rb)
}

func (c FfiConverterTreeExternalDiff) Lower(value TreeExternalDiff) C.RustBuffer {
	return LowerIntoRustBuffer[TreeExternalDiff](c, value)
}
func (FfiConverterTreeExternalDiff) Read(reader io.Reader) TreeExternalDiff {
	id := readInt32(reader)
	switch id {
	case 1:
		return TreeExternalDiffCreate{
			FfiConverterTreeParentIdINSTANCE.Read(reader),
			FfiConverterUint32INSTANCE.Read(reader),
			FfiConverterStringINSTANCE.Read(reader),
		}
	case 2:
		return TreeExternalDiffMove{
			FfiConverterTreeParentIdINSTANCE.Read(reader),
			FfiConverterUint32INSTANCE.Read(reader),
			FfiConverterStringINSTANCE.Read(reader),
			FfiConverterTreeParentIdINSTANCE.Read(reader),
			FfiConverterUint32INSTANCE.Read(reader),
		}
	case 3:
		return TreeExternalDiffDelete{
			FfiConverterTreeParentIdINSTANCE.Read(reader),
			FfiConverterUint32INSTANCE.Read(reader),
		}
	default:
		panic(fmt.Sprintf("invalid enum value %v in FfiConverterTreeExternalDiff.Read()", id))
	}
}

func (FfiConverterTreeExternalDiff) Write(writer io.Writer, value TreeExternalDiff) {
	switch variant_value := value.(type) {
	case TreeExternalDiffCreate:
		writeInt32(writer, 1)
		FfiConverterTreeParentIdINSTANCE.Write(writer, variant_value.Parent)
		FfiConverterUint32INSTANCE.Write(writer, variant_value.Index)
		FfiConverterStringINSTANCE.Write(writer, variant_value.FractionalIndex)
	case TreeExternalDiffMove:
		writeInt32(writer, 2)
		FfiConverterTreeParentIdINSTANCE.Write(writer, variant_value.Parent)
		FfiConverterUint32INSTANCE.Write(writer, variant_value.Index)
		FfiConverterStringINSTANCE.Write(writer, variant_value.FractionalIndex)
		FfiConverterTreeParentIdINSTANCE.Write(writer, variant_value.OldParent)
		FfiConverterUint32INSTANCE.Write(writer, variant_value.OldIndex)
	case TreeExternalDiffDelete:
		writeInt32(writer, 3)
		FfiConverterTreeParentIdINSTANCE.Write(writer, variant_value.OldParent)
		FfiConverterUint32INSTANCE.Write(writer, variant_value.OldIndex)
	default:
		_ = variant_value
		panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterTreeExternalDiff.Write", value))
	}
}

type FfiDestroyerTreeExternalDiff struct{}

func (_ FfiDestroyerTreeExternalDiff) Destroy(value TreeExternalDiff) {
	value.Destroy()
}

type TreeParentId interface {
	Destroy()
}
type TreeParentIdNode struct {
	Id TreeId
}

func (e TreeParentIdNode) Destroy() {
	FfiDestroyerTreeId{}.Destroy(e.Id)
}

type TreeParentIdRoot struct {
}

func (e TreeParentIdRoot) Destroy() {
}

type TreeParentIdDeleted struct {
}

func (e TreeParentIdDeleted) Destroy() {
}

type TreeParentIdUnexist struct {
}

func (e TreeParentIdUnexist) Destroy() {
}

type FfiConverterTreeParentId struct{}

var FfiConverterTreeParentIdINSTANCE = FfiConverterTreeParentId{}

func (c FfiConverterTreeParentId) Lift(rb RustBufferI) TreeParentId {
	return LiftFromRustBuffer[TreeParentId](c, rb)
}

func (c FfiConverterTreeParentId) Lower(value TreeParentId) C.RustBuffer {
	return LowerIntoRustBuffer[TreeParentId](c, value)
}
func (FfiConverterTreeParentId) Read(reader io.Reader) TreeParentId {
	id := readInt32(reader)
	switch id {
	case 1:
		return TreeParentIdNode{
			FfiConverterTreeIdINSTANCE.Read(reader),
		}
	case 2:
		return TreeParentIdRoot{}
	case 3:
		return TreeParentIdDeleted{}
	case 4:
		return TreeParentIdUnexist{}
	default:
		panic(fmt.Sprintf("invalid enum value %v in FfiConverterTreeParentId.Read()", id))
	}
}

func (FfiConverterTreeParentId) Write(writer io.Writer, value TreeParentId) {
	switch variant_value := value.(type) {
	case TreeParentIdNode:
		writeInt32(writer, 1)
		FfiConverterTreeIdINSTANCE.Write(writer, variant_value.Id)
	case TreeParentIdRoot:
		writeInt32(writer, 2)
	case TreeParentIdDeleted:
		writeInt32(writer, 3)
	case TreeParentIdUnexist:
		writeInt32(writer, 4)
	default:
		_ = variant_value
		panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterTreeParentId.Write", value))
	}
}

type FfiDestroyerTreeParentId struct{}

func (_ FfiDestroyerTreeParentId) Destroy(value TreeParentId) {
	value.Destroy()
}

type UndoOrRedo uint

const (
	UndoOrRedoUndo UndoOrRedo = 1
	UndoOrRedoRedo UndoOrRedo = 2
)

type FfiConverterUndoOrRedo struct{}

var FfiConverterUndoOrRedoINSTANCE = FfiConverterUndoOrRedo{}

func (c FfiConverterUndoOrRedo) Lift(rb RustBufferI) UndoOrRedo {
	return LiftFromRustBuffer[UndoOrRedo](c, rb)
}

func (c FfiConverterUndoOrRedo) Lower(value UndoOrRedo) C.RustBuffer {
	return LowerIntoRustBuffer[UndoOrRedo](c, value)
}
func (FfiConverterUndoOrRedo) Read(reader io.Reader) UndoOrRedo {
	id := readInt32(reader)
	return UndoOrRedo(id)
}

func (FfiConverterUndoOrRedo) Write(writer io.Writer, value UndoOrRedo) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerUndoOrRedo struct{}

func (_ FfiDestroyerUndoOrRedo) Destroy(value UndoOrRedo) {
}

type UpdateTimeoutError struct {
	err error
}

// Convience method to turn *UpdateTimeoutError into error
// Avoiding treating nil pointer as non nil error interface
func (err *UpdateTimeoutError) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err UpdateTimeoutError) Error() string {
	return fmt.Sprintf("UpdateTimeoutError: %s", err.err.Error())
}

func (err UpdateTimeoutError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrUpdateTimeoutErrorTimeout = fmt.Errorf("UpdateTimeoutErrorTimeout")

// Variant structs
type UpdateTimeoutErrorTimeout struct {
	message string
}

func NewUpdateTimeoutErrorTimeout() *UpdateTimeoutError {
	return &UpdateTimeoutError{err: &UpdateTimeoutErrorTimeout{}}
}

func (e UpdateTimeoutErrorTimeout) destroy() {
}

func (err UpdateTimeoutErrorTimeout) Error() string {
	return fmt.Sprintf("Timeout: %s", err.message)
}

func (self UpdateTimeoutErrorTimeout) Is(target error) bool {
	return target == ErrUpdateTimeoutErrorTimeout
}

type FfiConverterUpdateTimeoutError struct{}

var FfiConverterUpdateTimeoutErrorINSTANCE = FfiConverterUpdateTimeoutError{}

func (c FfiConverterUpdateTimeoutError) Lift(eb RustBufferI) *UpdateTimeoutError {
	return LiftFromRustBuffer[*UpdateTimeoutError](c, eb)
}

func (c FfiConverterUpdateTimeoutError) Lower(value *UpdateTimeoutError) C.RustBuffer {
	return LowerIntoRustBuffer[*UpdateTimeoutError](c, value)
}

func (c FfiConverterUpdateTimeoutError) Read(reader io.Reader) *UpdateTimeoutError {
	errorID := readUint32(reader)

	message := FfiConverterStringINSTANCE.Read(reader)
	switch errorID {
	case 1:
		return &UpdateTimeoutError{&UpdateTimeoutErrorTimeout{message}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterUpdateTimeoutError.Read()", errorID))
	}

}

func (c FfiConverterUpdateTimeoutError) Write(writer io.Writer, value *UpdateTimeoutError) {
	switch variantValue := value.err.(type) {
	case *UpdateTimeoutErrorTimeout:
		writeInt32(writer, 1)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterUpdateTimeoutError.Write", value))
	}
}

type FfiDestroyerUpdateTimeoutError struct{}

func (_ FfiDestroyerUpdateTimeoutError) Destroy(value *UpdateTimeoutError) {
	switch variantValue := value.err.(type) {
	case UpdateTimeoutErrorTimeout:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerUpdateTimeoutError.Destroy", value))
	}
}

type FfiConverterOptionalUint32 struct{}

var FfiConverterOptionalUint32INSTANCE = FfiConverterOptionalUint32{}

func (c FfiConverterOptionalUint32) Lift(rb RustBufferI) *uint32 {
	return LiftFromRustBuffer[*uint32](c, rb)
}

func (_ FfiConverterOptionalUint32) Read(reader io.Reader) *uint32 {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterUint32INSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalUint32) Lower(value *uint32) C.RustBuffer {
	return LowerIntoRustBuffer[*uint32](c, value)
}

func (_ FfiConverterOptionalUint32) Write(writer io.Writer, value *uint32) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterUint32INSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalUint32 struct{}

func (_ FfiDestroyerOptionalUint32) Destroy(value *uint32) {
	if value != nil {
		FfiDestroyerUint32{}.Destroy(*value)
	}
}

type FfiConverterOptionalInt32 struct{}

var FfiConverterOptionalInt32INSTANCE = FfiConverterOptionalInt32{}

func (c FfiConverterOptionalInt32) Lift(rb RustBufferI) *int32 {
	return LiftFromRustBuffer[*int32](c, rb)
}

func (_ FfiConverterOptionalInt32) Read(reader io.Reader) *int32 {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterInt32INSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalInt32) Lower(value *int32) C.RustBuffer {
	return LowerIntoRustBuffer[*int32](c, value)
}

func (_ FfiConverterOptionalInt32) Write(writer io.Writer, value *int32) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterInt32INSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalInt32 struct{}

func (_ FfiDestroyerOptionalInt32) Destroy(value *int32) {
	if value != nil {
		FfiDestroyerInt32{}.Destroy(*value)
	}
}

type FfiConverterOptionalUint64 struct{}

var FfiConverterOptionalUint64INSTANCE = FfiConverterOptionalUint64{}

func (c FfiConverterOptionalUint64) Lift(rb RustBufferI) *uint64 {
	return LiftFromRustBuffer[*uint64](c, rb)
}

func (_ FfiConverterOptionalUint64) Read(reader io.Reader) *uint64 {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterUint64INSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalUint64) Lower(value *uint64) C.RustBuffer {
	return LowerIntoRustBuffer[*uint64](c, value)
}

func (_ FfiConverterOptionalUint64) Write(writer io.Writer, value *uint64) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterUint64INSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalUint64 struct{}

func (_ FfiDestroyerOptionalUint64) Destroy(value *uint64) {
	if value != nil {
		FfiDestroyerUint64{}.Destroy(*value)
	}
}

type FfiConverterOptionalInt64 struct{}

var FfiConverterOptionalInt64INSTANCE = FfiConverterOptionalInt64{}

func (c FfiConverterOptionalInt64) Lift(rb RustBufferI) *int64 {
	return LiftFromRustBuffer[*int64](c, rb)
}

func (_ FfiConverterOptionalInt64) Read(reader io.Reader) *int64 {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterInt64INSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalInt64) Lower(value *int64) C.RustBuffer {
	return LowerIntoRustBuffer[*int64](c, value)
}

func (_ FfiConverterOptionalInt64) Write(writer io.Writer, value *int64) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterInt64INSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalInt64 struct{}

func (_ FfiDestroyerOptionalInt64) Destroy(value *int64) {
	if value != nil {
		FfiDestroyerInt64{}.Destroy(*value)
	}
}

type FfiConverterOptionalFloat64 struct{}

var FfiConverterOptionalFloat64INSTANCE = FfiConverterOptionalFloat64{}

func (c FfiConverterOptionalFloat64) Lift(rb RustBufferI) *float64 {
	return LiftFromRustBuffer[*float64](c, rb)
}

func (_ FfiConverterOptionalFloat64) Read(reader io.Reader) *float64 {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterFloat64INSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalFloat64) Lower(value *float64) C.RustBuffer {
	return LowerIntoRustBuffer[*float64](c, value)
}

func (_ FfiConverterOptionalFloat64) Write(writer io.Writer, value *float64) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterFloat64INSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalFloat64 struct{}

func (_ FfiDestroyerOptionalFloat64) Destroy(value *float64) {
	if value != nil {
		FfiDestroyerFloat64{}.Destroy(*value)
	}
}

type FfiConverterOptionalString struct{}

var FfiConverterOptionalStringINSTANCE = FfiConverterOptionalString{}

func (c FfiConverterOptionalString) Lift(rb RustBufferI) *string {
	return LiftFromRustBuffer[*string](c, rb)
}

func (_ FfiConverterOptionalString) Read(reader io.Reader) *string {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterStringINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalString) Lower(value *string) C.RustBuffer {
	return LowerIntoRustBuffer[*string](c, value)
}

func (_ FfiConverterOptionalString) Write(writer io.Writer, value *string) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterStringINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalString struct{}

func (_ FfiDestroyerOptionalString) Destroy(value *string) {
	if value != nil {
		FfiDestroyerString{}.Destroy(*value)
	}
}

type FfiConverterOptionalCursor struct{}

var FfiConverterOptionalCursorINSTANCE = FfiConverterOptionalCursor{}

func (c FfiConverterOptionalCursor) Lift(rb RustBufferI) **Cursor {
	return LiftFromRustBuffer[**Cursor](c, rb)
}

func (_ FfiConverterOptionalCursor) Read(reader io.Reader) **Cursor {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterCursorINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalCursor) Lower(value **Cursor) C.RustBuffer {
	return LowerIntoRustBuffer[**Cursor](c, value)
}

func (_ FfiConverterOptionalCursor) Write(writer io.Writer, value **Cursor) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterCursorINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalCursor struct{}

func (_ FfiDestroyerOptionalCursor) Destroy(value **Cursor) {
	if value != nil {
		FfiDestroyerCursor{}.Destroy(*value)
	}
}

type FfiConverterOptionalFrontiers struct{}

var FfiConverterOptionalFrontiersINSTANCE = FfiConverterOptionalFrontiers{}

func (c FfiConverterOptionalFrontiers) Lift(rb RustBufferI) **Frontiers {
	return LiftFromRustBuffer[**Frontiers](c, rb)
}

func (_ FfiConverterOptionalFrontiers) Read(reader io.Reader) **Frontiers {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterFrontiersINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalFrontiers) Lower(value **Frontiers) C.RustBuffer {
	return LowerIntoRustBuffer[**Frontiers](c, value)
}

func (_ FfiConverterOptionalFrontiers) Write(writer io.Writer, value **Frontiers) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterFrontiersINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalFrontiers struct{}

func (_ FfiDestroyerOptionalFrontiers) Destroy(value **Frontiers) {
	if value != nil {
		FfiDestroyerFrontiers{}.Destroy(*value)
	}
}

type FfiConverterOptionalLoroCounter struct{}

var FfiConverterOptionalLoroCounterINSTANCE = FfiConverterOptionalLoroCounter{}

func (c FfiConverterOptionalLoroCounter) Lift(rb RustBufferI) **LoroCounter {
	return LiftFromRustBuffer[**LoroCounter](c, rb)
}

func (_ FfiConverterOptionalLoroCounter) Read(reader io.Reader) **LoroCounter {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterLoroCounterINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalLoroCounter) Lower(value **LoroCounter) C.RustBuffer {
	return LowerIntoRustBuffer[**LoroCounter](c, value)
}

func (_ FfiConverterOptionalLoroCounter) Write(writer io.Writer, value **LoroCounter) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterLoroCounterINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalLoroCounter struct{}

func (_ FfiDestroyerOptionalLoroCounter) Destroy(value **LoroCounter) {
	if value != nil {
		FfiDestroyerLoroCounter{}.Destroy(*value)
	}
}

type FfiConverterOptionalLoroDoc struct{}

var FfiConverterOptionalLoroDocINSTANCE = FfiConverterOptionalLoroDoc{}

func (c FfiConverterOptionalLoroDoc) Lift(rb RustBufferI) **LoroDoc {
	return LiftFromRustBuffer[**LoroDoc](c, rb)
}

func (_ FfiConverterOptionalLoroDoc) Read(reader io.Reader) **LoroDoc {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterLoroDocINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalLoroDoc) Lower(value **LoroDoc) C.RustBuffer {
	return LowerIntoRustBuffer[**LoroDoc](c, value)
}

func (_ FfiConverterOptionalLoroDoc) Write(writer io.Writer, value **LoroDoc) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterLoroDocINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalLoroDoc struct{}

func (_ FfiDestroyerOptionalLoroDoc) Destroy(value **LoroDoc) {
	if value != nil {
		FfiDestroyerLoroDoc{}.Destroy(*value)
	}
}

type FfiConverterOptionalLoroList struct{}

var FfiConverterOptionalLoroListINSTANCE = FfiConverterOptionalLoroList{}

func (c FfiConverterOptionalLoroList) Lift(rb RustBufferI) **LoroList {
	return LiftFromRustBuffer[**LoroList](c, rb)
}

func (_ FfiConverterOptionalLoroList) Read(reader io.Reader) **LoroList {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterLoroListINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalLoroList) Lower(value **LoroList) C.RustBuffer {
	return LowerIntoRustBuffer[**LoroList](c, value)
}

func (_ FfiConverterOptionalLoroList) Write(writer io.Writer, value **LoroList) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterLoroListINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalLoroList struct{}

func (_ FfiDestroyerOptionalLoroList) Destroy(value **LoroList) {
	if value != nil {
		FfiDestroyerLoroList{}.Destroy(*value)
	}
}

type FfiConverterOptionalLoroMap struct{}

var FfiConverterOptionalLoroMapINSTANCE = FfiConverterOptionalLoroMap{}

func (c FfiConverterOptionalLoroMap) Lift(rb RustBufferI) **LoroMap {
	return LiftFromRustBuffer[**LoroMap](c, rb)
}

func (_ FfiConverterOptionalLoroMap) Read(reader io.Reader) **LoroMap {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterLoroMapINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalLoroMap) Lower(value **LoroMap) C.RustBuffer {
	return LowerIntoRustBuffer[**LoroMap](c, value)
}

func (_ FfiConverterOptionalLoroMap) Write(writer io.Writer, value **LoroMap) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterLoroMapINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalLoroMap struct{}

func (_ FfiDestroyerOptionalLoroMap) Destroy(value **LoroMap) {
	if value != nil {
		FfiDestroyerLoroMap{}.Destroy(*value)
	}
}

type FfiConverterOptionalLoroMovableList struct{}

var FfiConverterOptionalLoroMovableListINSTANCE = FfiConverterOptionalLoroMovableList{}

func (c FfiConverterOptionalLoroMovableList) Lift(rb RustBufferI) **LoroMovableList {
	return LiftFromRustBuffer[**LoroMovableList](c, rb)
}

func (_ FfiConverterOptionalLoroMovableList) Read(reader io.Reader) **LoroMovableList {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterLoroMovableListINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalLoroMovableList) Lower(value **LoroMovableList) C.RustBuffer {
	return LowerIntoRustBuffer[**LoroMovableList](c, value)
}

func (_ FfiConverterOptionalLoroMovableList) Write(writer io.Writer, value **LoroMovableList) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterLoroMovableListINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalLoroMovableList struct{}

func (_ FfiDestroyerOptionalLoroMovableList) Destroy(value **LoroMovableList) {
	if value != nil {
		FfiDestroyerLoroMovableList{}.Destroy(*value)
	}
}

type FfiConverterOptionalLoroText struct{}

var FfiConverterOptionalLoroTextINSTANCE = FfiConverterOptionalLoroText{}

func (c FfiConverterOptionalLoroText) Lift(rb RustBufferI) **LoroText {
	return LiftFromRustBuffer[**LoroText](c, rb)
}

func (_ FfiConverterOptionalLoroText) Read(reader io.Reader) **LoroText {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterLoroTextINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalLoroText) Lower(value **LoroText) C.RustBuffer {
	return LowerIntoRustBuffer[**LoroText](c, value)
}

func (_ FfiConverterOptionalLoroText) Write(writer io.Writer, value **LoroText) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterLoroTextINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalLoroText struct{}

func (_ FfiDestroyerOptionalLoroText) Destroy(value **LoroText) {
	if value != nil {
		FfiDestroyerLoroText{}.Destroy(*value)
	}
}

type FfiConverterOptionalLoroTree struct{}

var FfiConverterOptionalLoroTreeINSTANCE = FfiConverterOptionalLoroTree{}

func (c FfiConverterOptionalLoroTree) Lift(rb RustBufferI) **LoroTree {
	return LiftFromRustBuffer[**LoroTree](c, rb)
}

func (_ FfiConverterOptionalLoroTree) Read(reader io.Reader) **LoroTree {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterLoroTreeINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalLoroTree) Lower(value **LoroTree) C.RustBuffer {
	return LowerIntoRustBuffer[**LoroTree](c, value)
}

func (_ FfiConverterOptionalLoroTree) Write(writer io.Writer, value **LoroTree) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterLoroTreeINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalLoroTree struct{}

func (_ FfiDestroyerOptionalLoroTree) Destroy(value **LoroTree) {
	if value != nil {
		FfiDestroyerLoroTree{}.Destroy(*value)
	}
}

type FfiConverterOptionalLoroUnknown struct{}

var FfiConverterOptionalLoroUnknownINSTANCE = FfiConverterOptionalLoroUnknown{}

func (c FfiConverterOptionalLoroUnknown) Lift(rb RustBufferI) **LoroUnknown {
	return LiftFromRustBuffer[**LoroUnknown](c, rb)
}

func (_ FfiConverterOptionalLoroUnknown) Read(reader io.Reader) **LoroUnknown {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterLoroUnknownINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalLoroUnknown) Lower(value **LoroUnknown) C.RustBuffer {
	return LowerIntoRustBuffer[**LoroUnknown](c, value)
}

func (_ FfiConverterOptionalLoroUnknown) Write(writer io.Writer, value **LoroUnknown) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterLoroUnknownINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalLoroUnknown struct{}

func (_ FfiDestroyerOptionalLoroUnknown) Destroy(value **LoroUnknown) {
	if value != nil {
		FfiDestroyerLoroUnknown{}.Destroy(*value)
	}
}

type FfiConverterOptionalOnPop struct{}

var FfiConverterOptionalOnPopINSTANCE = FfiConverterOptionalOnPop{}

func (c FfiConverterOptionalOnPop) Lift(rb RustBufferI) *OnPop {
	return LiftFromRustBuffer[*OnPop](c, rb)
}

func (_ FfiConverterOptionalOnPop) Read(reader io.Reader) *OnPop {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterOnPopINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalOnPop) Lower(value *OnPop) C.RustBuffer {
	return LowerIntoRustBuffer[*OnPop](c, value)
}

func (_ FfiConverterOptionalOnPop) Write(writer io.Writer, value *OnPop) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterOnPopINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalOnPop struct{}

func (_ FfiDestroyerOptionalOnPop) Destroy(value *OnPop) {
	if value != nil {
		FfiDestroyerOnPop{}.Destroy(*value)
	}
}

type FfiConverterOptionalOnPush struct{}

var FfiConverterOptionalOnPushINSTANCE = FfiConverterOptionalOnPush{}

func (c FfiConverterOptionalOnPush) Lift(rb RustBufferI) *OnPush {
	return LiftFromRustBuffer[*OnPush](c, rb)
}

func (_ FfiConverterOptionalOnPush) Read(reader io.Reader) *OnPush {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterOnPushINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalOnPush) Lower(value *OnPush) C.RustBuffer {
	return LowerIntoRustBuffer[*OnPush](c, value)
}

func (_ FfiConverterOptionalOnPush) Write(writer io.Writer, value *OnPush) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterOnPushINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalOnPush struct{}

func (_ FfiDestroyerOptionalOnPush) Destroy(value *OnPush) {
	if value != nil {
		FfiDestroyerOnPush{}.Destroy(*value)
	}
}

type FfiConverterOptionalValueOrContainer struct{}

var FfiConverterOptionalValueOrContainerINSTANCE = FfiConverterOptionalValueOrContainer{}

func (c FfiConverterOptionalValueOrContainer) Lift(rb RustBufferI) **ValueOrContainer {
	return LiftFromRustBuffer[**ValueOrContainer](c, rb)
}

func (_ FfiConverterOptionalValueOrContainer) Read(reader io.Reader) **ValueOrContainer {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterValueOrContainerINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalValueOrContainer) Lower(value **ValueOrContainer) C.RustBuffer {
	return LowerIntoRustBuffer[**ValueOrContainer](c, value)
}

func (_ FfiConverterOptionalValueOrContainer) Write(writer io.Writer, value **ValueOrContainer) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterValueOrContainerINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalValueOrContainer struct{}

func (_ FfiDestroyerOptionalValueOrContainer) Destroy(value **ValueOrContainer) {
	if value != nil {
		FfiDestroyerValueOrContainer{}.Destroy(*value)
	}
}

type FfiConverterOptionalVersionVector struct{}

var FfiConverterOptionalVersionVectorINSTANCE = FfiConverterOptionalVersionVector{}

func (c FfiConverterOptionalVersionVector) Lift(rb RustBufferI) **VersionVector {
	return LiftFromRustBuffer[**VersionVector](c, rb)
}

func (_ FfiConverterOptionalVersionVector) Read(reader io.Reader) **VersionVector {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterVersionVectorINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalVersionVector) Lower(value **VersionVector) C.RustBuffer {
	return LowerIntoRustBuffer[**VersionVector](c, value)
}

func (_ FfiConverterOptionalVersionVector) Write(writer io.Writer, value **VersionVector) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterVersionVectorINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalVersionVector struct{}

func (_ FfiDestroyerOptionalVersionVector) Destroy(value **VersionVector) {
	if value != nil {
		FfiDestroyerVersionVector{}.Destroy(*value)
	}
}

type FfiConverterOptionalChangeMeta struct{}

var FfiConverterOptionalChangeMetaINSTANCE = FfiConverterOptionalChangeMeta{}

func (c FfiConverterOptionalChangeMeta) Lift(rb RustBufferI) *ChangeMeta {
	return LiftFromRustBuffer[*ChangeMeta](c, rb)
}

func (_ FfiConverterOptionalChangeMeta) Read(reader io.Reader) *ChangeMeta {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterChangeMetaINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalChangeMeta) Lower(value *ChangeMeta) C.RustBuffer {
	return LowerIntoRustBuffer[*ChangeMeta](c, value)
}

func (_ FfiConverterOptionalChangeMeta) Write(writer io.Writer, value *ChangeMeta) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterChangeMetaINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalChangeMeta struct{}

func (_ FfiDestroyerOptionalChangeMeta) Destroy(value *ChangeMeta) {
	if value != nil {
		FfiDestroyerChangeMeta{}.Destroy(*value)
	}
}

type FfiConverterOptionalCounterSpan struct{}

var FfiConverterOptionalCounterSpanINSTANCE = FfiConverterOptionalCounterSpan{}

func (c FfiConverterOptionalCounterSpan) Lift(rb RustBufferI) *CounterSpan {
	return LiftFromRustBuffer[*CounterSpan](c, rb)
}

func (_ FfiConverterOptionalCounterSpan) Read(reader io.Reader) *CounterSpan {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterCounterSpanINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalCounterSpan) Lower(value *CounterSpan) C.RustBuffer {
	return LowerIntoRustBuffer[*CounterSpan](c, value)
}

func (_ FfiConverterOptionalCounterSpan) Write(writer io.Writer, value *CounterSpan) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterCounterSpanINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalCounterSpan struct{}

func (_ FfiDestroyerOptionalCounterSpan) Destroy(value *CounterSpan) {
	if value != nil {
		FfiDestroyerCounterSpan{}.Destroy(*value)
	}
}

type FfiConverterOptionalDiffEvent struct{}

var FfiConverterOptionalDiffEventINSTANCE = FfiConverterOptionalDiffEvent{}

func (c FfiConverterOptionalDiffEvent) Lift(rb RustBufferI) *DiffEvent {
	return LiftFromRustBuffer[*DiffEvent](c, rb)
}

func (_ FfiConverterOptionalDiffEvent) Read(reader io.Reader) *DiffEvent {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterDiffEventINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalDiffEvent) Lower(value *DiffEvent) C.RustBuffer {
	return LowerIntoRustBuffer[*DiffEvent](c, value)
}

func (_ FfiConverterOptionalDiffEvent) Write(writer io.Writer, value *DiffEvent) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterDiffEventINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalDiffEvent struct{}

func (_ FfiDestroyerOptionalDiffEvent) Destroy(value *DiffEvent) {
	if value != nil {
		FfiDestroyerDiffEvent{}.Destroy(*value)
	}
}

type FfiConverterOptionalId struct{}

var FfiConverterOptionalIdINSTANCE = FfiConverterOptionalId{}

func (c FfiConverterOptionalId) Lift(rb RustBufferI) *Id {
	return LiftFromRustBuffer[*Id](c, rb)
}

func (_ FfiConverterOptionalId) Read(reader io.Reader) *Id {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterIdINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalId) Lower(value *Id) C.RustBuffer {
	return LowerIntoRustBuffer[*Id](c, value)
}

func (_ FfiConverterOptionalId) Write(writer io.Writer, value *Id) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterIdINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalId struct{}

func (_ FfiDestroyerOptionalId) Destroy(value *Id) {
	if value != nil {
		FfiDestroyerId{}.Destroy(*value)
	}
}

type FfiConverterOptionalStyleConfig struct{}

var FfiConverterOptionalStyleConfigINSTANCE = FfiConverterOptionalStyleConfig{}

func (c FfiConverterOptionalStyleConfig) Lift(rb RustBufferI) *StyleConfig {
	return LiftFromRustBuffer[*StyleConfig](c, rb)
}

func (_ FfiConverterOptionalStyleConfig) Read(reader io.Reader) *StyleConfig {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterStyleConfigINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalStyleConfig) Lower(value *StyleConfig) C.RustBuffer {
	return LowerIntoRustBuffer[*StyleConfig](c, value)
}

func (_ FfiConverterOptionalStyleConfig) Write(writer io.Writer, value *StyleConfig) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterStyleConfigINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalStyleConfig struct{}

func (_ FfiDestroyerOptionalStyleConfig) Destroy(value *StyleConfig) {
	if value != nil {
		FfiDestroyerStyleConfig{}.Destroy(*value)
	}
}

type FfiConverterOptionalContainerId struct{}

var FfiConverterOptionalContainerIdINSTANCE = FfiConverterOptionalContainerId{}

func (c FfiConverterOptionalContainerId) Lift(rb RustBufferI) *ContainerId {
	return LiftFromRustBuffer[*ContainerId](c, rb)
}

func (_ FfiConverterOptionalContainerId) Read(reader io.Reader) *ContainerId {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterContainerIdINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalContainerId) Lower(value *ContainerId) C.RustBuffer {
	return LowerIntoRustBuffer[*ContainerId](c, value)
}

func (_ FfiConverterOptionalContainerId) Write(writer io.Writer, value *ContainerId) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterContainerIdINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalContainerId struct{}

func (_ FfiDestroyerOptionalContainerId) Destroy(value *ContainerId) {
	if value != nil {
		FfiDestroyerContainerId{}.Destroy(*value)
	}
}

type FfiConverterOptionalContainerType struct{}

var FfiConverterOptionalContainerTypeINSTANCE = FfiConverterOptionalContainerType{}

func (c FfiConverterOptionalContainerType) Lift(rb RustBufferI) *ContainerType {
	return LiftFromRustBuffer[*ContainerType](c, rb)
}

func (_ FfiConverterOptionalContainerType) Read(reader io.Reader) *ContainerType {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterContainerTypeINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalContainerType) Lower(value *ContainerType) C.RustBuffer {
	return LowerIntoRustBuffer[*ContainerType](c, value)
}

func (_ FfiConverterOptionalContainerType) Write(writer io.Writer, value *ContainerType) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterContainerTypeINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalContainerType struct{}

func (_ FfiDestroyerOptionalContainerType) Destroy(value *ContainerType) {
	if value != nil {
		FfiDestroyerContainerType{}.Destroy(*value)
	}
}

type FfiConverterOptionalDiff struct{}

var FfiConverterOptionalDiffINSTANCE = FfiConverterOptionalDiff{}

func (c FfiConverterOptionalDiff) Lift(rb RustBufferI) *Diff {
	return LiftFromRustBuffer[*Diff](c, rb)
}

func (_ FfiConverterOptionalDiff) Read(reader io.Reader) *Diff {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterDiffINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalDiff) Lower(value *Diff) C.RustBuffer {
	return LowerIntoRustBuffer[*Diff](c, value)
}

func (_ FfiConverterOptionalDiff) Write(writer io.Writer, value *Diff) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterDiffINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalDiff struct{}

func (_ FfiDestroyerOptionalDiff) Destroy(value *Diff) {
	if value != nil {
		FfiDestroyerDiff{}.Destroy(*value)
	}
}

type FfiConverterOptionalLoroValue struct{}

var FfiConverterOptionalLoroValueINSTANCE = FfiConverterOptionalLoroValue{}

func (c FfiConverterOptionalLoroValue) Lift(rb RustBufferI) *LoroValue {
	return LiftFromRustBuffer[*LoroValue](c, rb)
}

func (_ FfiConverterOptionalLoroValue) Read(reader io.Reader) *LoroValue {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterLoroValueINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalLoroValue) Lower(value *LoroValue) C.RustBuffer {
	return LowerIntoRustBuffer[*LoroValue](c, value)
}

func (_ FfiConverterOptionalLoroValue) Write(writer io.Writer, value *LoroValue) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterLoroValueINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalLoroValue struct{}

func (_ FfiDestroyerOptionalLoroValue) Destroy(value *LoroValue) {
	if value != nil {
		FfiDestroyerLoroValue{}.Destroy(*value)
	}
}

type FfiConverterOptionalOrdering struct{}

var FfiConverterOptionalOrderingINSTANCE = FfiConverterOptionalOrdering{}

func (c FfiConverterOptionalOrdering) Lift(rb RustBufferI) *Ordering {
	return LiftFromRustBuffer[*Ordering](c, rb)
}

func (_ FfiConverterOptionalOrdering) Read(reader io.Reader) *Ordering {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterOrderingINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalOrdering) Lower(value *Ordering) C.RustBuffer {
	return LowerIntoRustBuffer[*Ordering](c, value)
}

func (_ FfiConverterOptionalOrdering) Write(writer io.Writer, value *Ordering) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterOrderingINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalOrdering struct{}

func (_ FfiDestroyerOptionalOrdering) Destroy(value *Ordering) {
	if value != nil {
		FfiDestroyerOrdering{}.Destroy(*value)
	}
}

type FfiConverterOptionalSequenceContainerPath struct{}

var FfiConverterOptionalSequenceContainerPathINSTANCE = FfiConverterOptionalSequenceContainerPath{}

func (c FfiConverterOptionalSequenceContainerPath) Lift(rb RustBufferI) *[]ContainerPath {
	return LiftFromRustBuffer[*[]ContainerPath](c, rb)
}

func (_ FfiConverterOptionalSequenceContainerPath) Read(reader io.Reader) *[]ContainerPath {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterSequenceContainerPathINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalSequenceContainerPath) Lower(value *[]ContainerPath) C.RustBuffer {
	return LowerIntoRustBuffer[*[]ContainerPath](c, value)
}

func (_ FfiConverterOptionalSequenceContainerPath) Write(writer io.Writer, value *[]ContainerPath) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterSequenceContainerPathINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalSequenceContainerPath struct{}

func (_ FfiDestroyerOptionalSequenceContainerPath) Destroy(value *[]ContainerPath) {
	if value != nil {
		FfiDestroyerSequenceContainerPath{}.Destroy(*value)
	}
}

type FfiConverterOptionalSequenceTreeId struct{}

var FfiConverterOptionalSequenceTreeIdINSTANCE = FfiConverterOptionalSequenceTreeId{}

func (c FfiConverterOptionalSequenceTreeId) Lift(rb RustBufferI) *[]TreeId {
	return LiftFromRustBuffer[*[]TreeId](c, rb)
}

func (_ FfiConverterOptionalSequenceTreeId) Read(reader io.Reader) *[]TreeId {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterSequenceTreeIdINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalSequenceTreeId) Lower(value *[]TreeId) C.RustBuffer {
	return LowerIntoRustBuffer[*[]TreeId](c, value)
}

func (_ FfiConverterOptionalSequenceTreeId) Write(writer io.Writer, value *[]TreeId) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterSequenceTreeIdINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalSequenceTreeId struct{}

func (_ FfiDestroyerOptionalSequenceTreeId) Destroy(value *[]TreeId) {
	if value != nil {
		FfiDestroyerSequenceTreeId{}.Destroy(*value)
	}
}

type FfiConverterOptionalMapUint64CounterSpan struct{}

var FfiConverterOptionalMapUint64CounterSpanINSTANCE = FfiConverterOptionalMapUint64CounterSpan{}

func (c FfiConverterOptionalMapUint64CounterSpan) Lift(rb RustBufferI) *map[uint64]CounterSpan {
	return LiftFromRustBuffer[*map[uint64]CounterSpan](c, rb)
}

func (_ FfiConverterOptionalMapUint64CounterSpan) Read(reader io.Reader) *map[uint64]CounterSpan {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterMapUint64CounterSpanINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalMapUint64CounterSpan) Lower(value *map[uint64]CounterSpan) C.RustBuffer {
	return LowerIntoRustBuffer[*map[uint64]CounterSpan](c, value)
}

func (_ FfiConverterOptionalMapUint64CounterSpan) Write(writer io.Writer, value *map[uint64]CounterSpan) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterMapUint64CounterSpanINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalMapUint64CounterSpan struct{}

func (_ FfiDestroyerOptionalMapUint64CounterSpan) Destroy(value *map[uint64]CounterSpan) {
	if value != nil {
		FfiDestroyerMapUint64CounterSpan{}.Destroy(*value)
	}
}

type FfiConverterOptionalMapStringLoroValue struct{}

var FfiConverterOptionalMapStringLoroValueINSTANCE = FfiConverterOptionalMapStringLoroValue{}

func (c FfiConverterOptionalMapStringLoroValue) Lift(rb RustBufferI) *map[string]LoroValue {
	return LiftFromRustBuffer[*map[string]LoroValue](c, rb)
}

func (_ FfiConverterOptionalMapStringLoroValue) Read(reader io.Reader) *map[string]LoroValue {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterMapStringLoroValueINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalMapStringLoroValue) Lower(value *map[string]LoroValue) C.RustBuffer {
	return LowerIntoRustBuffer[*map[string]LoroValue](c, value)
}

func (_ FfiConverterOptionalMapStringLoroValue) Write(writer io.Writer, value *map[string]LoroValue) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterMapStringLoroValueINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalMapStringLoroValue struct{}

func (_ FfiDestroyerOptionalMapStringLoroValue) Destroy(value *map[string]LoroValue) {
	if value != nil {
		FfiDestroyerMapStringLoroValue{}.Destroy(*value)
	}
}

type FfiConverterSequenceUint64 struct{}

var FfiConverterSequenceUint64INSTANCE = FfiConverterSequenceUint64{}

func (c FfiConverterSequenceUint64) Lift(rb RustBufferI) []uint64 {
	return LiftFromRustBuffer[[]uint64](c, rb)
}

func (c FfiConverterSequenceUint64) Read(reader io.Reader) []uint64 {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]uint64, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterUint64INSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceUint64) Lower(value []uint64) C.RustBuffer {
	return LowerIntoRustBuffer[[]uint64](c, value)
}

func (c FfiConverterSequenceUint64) Write(writer io.Writer, value []uint64) {
	if len(value) > math.MaxInt32 {
		panic("[]uint64 is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterUint64INSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceUint64 struct{}

func (FfiDestroyerSequenceUint64) Destroy(sequence []uint64) {
	for _, value := range sequence {
		FfiDestroyerUint64{}.Destroy(value)
	}
}

type FfiConverterSequenceString struct{}

var FfiConverterSequenceStringINSTANCE = FfiConverterSequenceString{}

func (c FfiConverterSequenceString) Lift(rb RustBufferI) []string {
	return LiftFromRustBuffer[[]string](c, rb)
}

func (c FfiConverterSequenceString) Read(reader io.Reader) []string {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]string, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterStringINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceString) Lower(value []string) C.RustBuffer {
	return LowerIntoRustBuffer[[]string](c, value)
}

func (c FfiConverterSequenceString) Write(writer io.Writer, value []string) {
	if len(value) > math.MaxInt32 {
		panic("[]string is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterStringINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceString struct{}

func (FfiDestroyerSequenceString) Destroy(sequence []string) {
	for _, value := range sequence {
		FfiDestroyerString{}.Destroy(value)
	}
}

type FfiConverterSequenceBytes struct{}

var FfiConverterSequenceBytesINSTANCE = FfiConverterSequenceBytes{}

func (c FfiConverterSequenceBytes) Lift(rb RustBufferI) [][]byte {
	return LiftFromRustBuffer[[][]byte](c, rb)
}

func (c FfiConverterSequenceBytes) Read(reader io.Reader) [][]byte {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([][]byte, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterBytesINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceBytes) Lower(value [][]byte) C.RustBuffer {
	return LowerIntoRustBuffer[[][]byte](c, value)
}

func (c FfiConverterSequenceBytes) Write(writer io.Writer, value [][]byte) {
	if len(value) > math.MaxInt32 {
		panic("[][]byte is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterBytesINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceBytes struct{}

func (FfiDestroyerSequenceBytes) Destroy(sequence [][]byte) {
	for _, value := range sequence {
		FfiDestroyerBytes{}.Destroy(value)
	}
}

type FfiConverterSequenceValueOrContainer struct{}

var FfiConverterSequenceValueOrContainerINSTANCE = FfiConverterSequenceValueOrContainer{}

func (c FfiConverterSequenceValueOrContainer) Lift(rb RustBufferI) []*ValueOrContainer {
	return LiftFromRustBuffer[[]*ValueOrContainer](c, rb)
}

func (c FfiConverterSequenceValueOrContainer) Read(reader io.Reader) []*ValueOrContainer {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]*ValueOrContainer, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterValueOrContainerINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceValueOrContainer) Lower(value []*ValueOrContainer) C.RustBuffer {
	return LowerIntoRustBuffer[[]*ValueOrContainer](c, value)
}

func (c FfiConverterSequenceValueOrContainer) Write(writer io.Writer, value []*ValueOrContainer) {
	if len(value) > math.MaxInt32 {
		panic("[]*ValueOrContainer is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterValueOrContainerINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceValueOrContainer struct{}

func (FfiDestroyerSequenceValueOrContainer) Destroy(sequence []*ValueOrContainer) {
	for _, value := range sequence {
		FfiDestroyerValueOrContainer{}.Destroy(value)
	}
}

type FfiConverterSequenceContainerDiff struct{}

var FfiConverterSequenceContainerDiffINSTANCE = FfiConverterSequenceContainerDiff{}

func (c FfiConverterSequenceContainerDiff) Lift(rb RustBufferI) []ContainerDiff {
	return LiftFromRustBuffer[[]ContainerDiff](c, rb)
}

func (c FfiConverterSequenceContainerDiff) Read(reader io.Reader) []ContainerDiff {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]ContainerDiff, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterContainerDiffINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceContainerDiff) Lower(value []ContainerDiff) C.RustBuffer {
	return LowerIntoRustBuffer[[]ContainerDiff](c, value)
}

func (c FfiConverterSequenceContainerDiff) Write(writer io.Writer, value []ContainerDiff) {
	if len(value) > math.MaxInt32 {
		panic("[]ContainerDiff is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterContainerDiffINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceContainerDiff struct{}

func (FfiDestroyerSequenceContainerDiff) Destroy(sequence []ContainerDiff) {
	for _, value := range sequence {
		FfiDestroyerContainerDiff{}.Destroy(value)
	}
}

type FfiConverterSequenceContainerIdAndDiff struct{}

var FfiConverterSequenceContainerIdAndDiffINSTANCE = FfiConverterSequenceContainerIdAndDiff{}

func (c FfiConverterSequenceContainerIdAndDiff) Lift(rb RustBufferI) []ContainerIdAndDiff {
	return LiftFromRustBuffer[[]ContainerIdAndDiff](c, rb)
}

func (c FfiConverterSequenceContainerIdAndDiff) Read(reader io.Reader) []ContainerIdAndDiff {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]ContainerIdAndDiff, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterContainerIdAndDiffINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceContainerIdAndDiff) Lower(value []ContainerIdAndDiff) C.RustBuffer {
	return LowerIntoRustBuffer[[]ContainerIdAndDiff](c, value)
}

func (c FfiConverterSequenceContainerIdAndDiff) Write(writer io.Writer, value []ContainerIdAndDiff) {
	if len(value) > math.MaxInt32 {
		panic("[]ContainerIdAndDiff is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterContainerIdAndDiffINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceContainerIdAndDiff struct{}

func (FfiDestroyerSequenceContainerIdAndDiff) Destroy(sequence []ContainerIdAndDiff) {
	for _, value := range sequence {
		FfiDestroyerContainerIdAndDiff{}.Destroy(value)
	}
}

type FfiConverterSequenceContainerPath struct{}

var FfiConverterSequenceContainerPathINSTANCE = FfiConverterSequenceContainerPath{}

func (c FfiConverterSequenceContainerPath) Lift(rb RustBufferI) []ContainerPath {
	return LiftFromRustBuffer[[]ContainerPath](c, rb)
}

func (c FfiConverterSequenceContainerPath) Read(reader io.Reader) []ContainerPath {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]ContainerPath, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterContainerPathINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceContainerPath) Lower(value []ContainerPath) C.RustBuffer {
	return LowerIntoRustBuffer[[]ContainerPath](c, value)
}

func (c FfiConverterSequenceContainerPath) Write(writer io.Writer, value []ContainerPath) {
	if len(value) > math.MaxInt32 {
		panic("[]ContainerPath is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterContainerPathINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceContainerPath struct{}

func (FfiDestroyerSequenceContainerPath) Destroy(sequence []ContainerPath) {
	for _, value := range sequence {
		FfiDestroyerContainerPath{}.Destroy(value)
	}
}

type FfiConverterSequenceCursorWithPos struct{}

var FfiConverterSequenceCursorWithPosINSTANCE = FfiConverterSequenceCursorWithPos{}

func (c FfiConverterSequenceCursorWithPos) Lift(rb RustBufferI) []CursorWithPos {
	return LiftFromRustBuffer[[]CursorWithPos](c, rb)
}

func (c FfiConverterSequenceCursorWithPos) Read(reader io.Reader) []CursorWithPos {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]CursorWithPos, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterCursorWithPosINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceCursorWithPos) Lower(value []CursorWithPos) C.RustBuffer {
	return LowerIntoRustBuffer[[]CursorWithPos](c, value)
}

func (c FfiConverterSequenceCursorWithPos) Write(writer io.Writer, value []CursorWithPos) {
	if len(value) > math.MaxInt32 {
		panic("[]CursorWithPos is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterCursorWithPosINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceCursorWithPos struct{}

func (FfiDestroyerSequenceCursorWithPos) Destroy(sequence []CursorWithPos) {
	for _, value := range sequence {
		FfiDestroyerCursorWithPos{}.Destroy(value)
	}
}

type FfiConverterSequenceId struct{}

var FfiConverterSequenceIdINSTANCE = FfiConverterSequenceId{}

func (c FfiConverterSequenceId) Lift(rb RustBufferI) []Id {
	return LiftFromRustBuffer[[]Id](c, rb)
}

func (c FfiConverterSequenceId) Read(reader io.Reader) []Id {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]Id, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterIdINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceId) Lower(value []Id) C.RustBuffer {
	return LowerIntoRustBuffer[[]Id](c, value)
}

func (c FfiConverterSequenceId) Write(writer io.Writer, value []Id) {
	if len(value) > math.MaxInt32 {
		panic("[]Id is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterIdINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceId struct{}

func (FfiDestroyerSequenceId) Destroy(sequence []Id) {
	for _, value := range sequence {
		FfiDestroyerId{}.Destroy(value)
	}
}

type FfiConverterSequenceIdSpan struct{}

var FfiConverterSequenceIdSpanINSTANCE = FfiConverterSequenceIdSpan{}

func (c FfiConverterSequenceIdSpan) Lift(rb RustBufferI) []IdSpan {
	return LiftFromRustBuffer[[]IdSpan](c, rb)
}

func (c FfiConverterSequenceIdSpan) Read(reader io.Reader) []IdSpan {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]IdSpan, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterIdSpanINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceIdSpan) Lower(value []IdSpan) C.RustBuffer {
	return LowerIntoRustBuffer[[]IdSpan](c, value)
}

func (c FfiConverterSequenceIdSpan) Write(writer io.Writer, value []IdSpan) {
	if len(value) > math.MaxInt32 {
		panic("[]IdSpan is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterIdSpanINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceIdSpan struct{}

func (FfiDestroyerSequenceIdSpan) Destroy(sequence []IdSpan) {
	for _, value := range sequence {
		FfiDestroyerIdSpan{}.Destroy(value)
	}
}

type FfiConverterSequencePathItem struct{}

var FfiConverterSequencePathItemINSTANCE = FfiConverterSequencePathItem{}

func (c FfiConverterSequencePathItem) Lift(rb RustBufferI) []PathItem {
	return LiftFromRustBuffer[[]PathItem](c, rb)
}

func (c FfiConverterSequencePathItem) Read(reader io.Reader) []PathItem {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]PathItem, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterPathItemINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequencePathItem) Lower(value []PathItem) C.RustBuffer {
	return LowerIntoRustBuffer[[]PathItem](c, value)
}

func (c FfiConverterSequencePathItem) Write(writer io.Writer, value []PathItem) {
	if len(value) > math.MaxInt32 {
		panic("[]PathItem is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterPathItemINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequencePathItem struct{}

func (FfiDestroyerSequencePathItem) Destroy(sequence []PathItem) {
	for _, value := range sequence {
		FfiDestroyerPathItem{}.Destroy(value)
	}
}

type FfiConverterSequenceTreeDiffItem struct{}

var FfiConverterSequenceTreeDiffItemINSTANCE = FfiConverterSequenceTreeDiffItem{}

func (c FfiConverterSequenceTreeDiffItem) Lift(rb RustBufferI) []TreeDiffItem {
	return LiftFromRustBuffer[[]TreeDiffItem](c, rb)
}

func (c FfiConverterSequenceTreeDiffItem) Read(reader io.Reader) []TreeDiffItem {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]TreeDiffItem, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterTreeDiffItemINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceTreeDiffItem) Lower(value []TreeDiffItem) C.RustBuffer {
	return LowerIntoRustBuffer[[]TreeDiffItem](c, value)
}

func (c FfiConverterSequenceTreeDiffItem) Write(writer io.Writer, value []TreeDiffItem) {
	if len(value) > math.MaxInt32 {
		panic("[]TreeDiffItem is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterTreeDiffItemINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceTreeDiffItem struct{}

func (FfiDestroyerSequenceTreeDiffItem) Destroy(sequence []TreeDiffItem) {
	for _, value := range sequence {
		FfiDestroyerTreeDiffItem{}.Destroy(value)
	}
}

type FfiConverterSequenceTreeId struct{}

var FfiConverterSequenceTreeIdINSTANCE = FfiConverterSequenceTreeId{}

func (c FfiConverterSequenceTreeId) Lift(rb RustBufferI) []TreeId {
	return LiftFromRustBuffer[[]TreeId](c, rb)
}

func (c FfiConverterSequenceTreeId) Read(reader io.Reader) []TreeId {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]TreeId, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterTreeIdINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceTreeId) Lower(value []TreeId) C.RustBuffer {
	return LowerIntoRustBuffer[[]TreeId](c, value)
}

func (c FfiConverterSequenceTreeId) Write(writer io.Writer, value []TreeId) {
	if len(value) > math.MaxInt32 {
		panic("[]TreeId is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterTreeIdINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceTreeId struct{}

func (FfiDestroyerSequenceTreeId) Destroy(sequence []TreeId) {
	for _, value := range sequence {
		FfiDestroyerTreeId{}.Destroy(value)
	}
}

type FfiConverterSequenceContainerId struct{}

var FfiConverterSequenceContainerIdINSTANCE = FfiConverterSequenceContainerId{}

func (c FfiConverterSequenceContainerId) Lift(rb RustBufferI) []ContainerId {
	return LiftFromRustBuffer[[]ContainerId](c, rb)
}

func (c FfiConverterSequenceContainerId) Read(reader io.Reader) []ContainerId {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]ContainerId, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterContainerIdINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceContainerId) Lower(value []ContainerId) C.RustBuffer {
	return LowerIntoRustBuffer[[]ContainerId](c, value)
}

func (c FfiConverterSequenceContainerId) Write(writer io.Writer, value []ContainerId) {
	if len(value) > math.MaxInt32 {
		panic("[]ContainerId is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterContainerIdINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceContainerId struct{}

func (FfiDestroyerSequenceContainerId) Destroy(sequence []ContainerId) {
	for _, value := range sequence {
		FfiDestroyerContainerId{}.Destroy(value)
	}
}

type FfiConverterSequenceIndex struct{}

var FfiConverterSequenceIndexINSTANCE = FfiConverterSequenceIndex{}

func (c FfiConverterSequenceIndex) Lift(rb RustBufferI) []Index {
	return LiftFromRustBuffer[[]Index](c, rb)
}

func (c FfiConverterSequenceIndex) Read(reader io.Reader) []Index {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]Index, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterIndexINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceIndex) Lower(value []Index) C.RustBuffer {
	return LowerIntoRustBuffer[[]Index](c, value)
}

func (c FfiConverterSequenceIndex) Write(writer io.Writer, value []Index) {
	if len(value) > math.MaxInt32 {
		panic("[]Index is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterIndexINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceIndex struct{}

func (FfiDestroyerSequenceIndex) Destroy(sequence []Index) {
	for _, value := range sequence {
		FfiDestroyerIndex{}.Destroy(value)
	}
}

type FfiConverterSequenceListDiffItem struct{}

var FfiConverterSequenceListDiffItemINSTANCE = FfiConverterSequenceListDiffItem{}

func (c FfiConverterSequenceListDiffItem) Lift(rb RustBufferI) []ListDiffItem {
	return LiftFromRustBuffer[[]ListDiffItem](c, rb)
}

func (c FfiConverterSequenceListDiffItem) Read(reader io.Reader) []ListDiffItem {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]ListDiffItem, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterListDiffItemINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceListDiffItem) Lower(value []ListDiffItem) C.RustBuffer {
	return LowerIntoRustBuffer[[]ListDiffItem](c, value)
}

func (c FfiConverterSequenceListDiffItem) Write(writer io.Writer, value []ListDiffItem) {
	if len(value) > math.MaxInt32 {
		panic("[]ListDiffItem is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterListDiffItemINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceListDiffItem struct{}

func (FfiDestroyerSequenceListDiffItem) Destroy(sequence []ListDiffItem) {
	for _, value := range sequence {
		FfiDestroyerListDiffItem{}.Destroy(value)
	}
}

type FfiConverterSequenceLoroValue struct{}

var FfiConverterSequenceLoroValueINSTANCE = FfiConverterSequenceLoroValue{}

func (c FfiConverterSequenceLoroValue) Lift(rb RustBufferI) []LoroValue {
	return LiftFromRustBuffer[[]LoroValue](c, rb)
}

func (c FfiConverterSequenceLoroValue) Read(reader io.Reader) []LoroValue {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]LoroValue, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterLoroValueINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceLoroValue) Lower(value []LoroValue) C.RustBuffer {
	return LowerIntoRustBuffer[[]LoroValue](c, value)
}

func (c FfiConverterSequenceLoroValue) Write(writer io.Writer, value []LoroValue) {
	if len(value) > math.MaxInt32 {
		panic("[]LoroValue is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterLoroValueINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceLoroValue struct{}

func (FfiDestroyerSequenceLoroValue) Destroy(sequence []LoroValue) {
	for _, value := range sequence {
		FfiDestroyerLoroValue{}.Destroy(value)
	}
}

type FfiConverterSequenceTextDelta struct{}

var FfiConverterSequenceTextDeltaINSTANCE = FfiConverterSequenceTextDelta{}

func (c FfiConverterSequenceTextDelta) Lift(rb RustBufferI) []TextDelta {
	return LiftFromRustBuffer[[]TextDelta](c, rb)
}

func (c FfiConverterSequenceTextDelta) Read(reader io.Reader) []TextDelta {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]TextDelta, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterTextDeltaINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceTextDelta) Lower(value []TextDelta) C.RustBuffer {
	return LowerIntoRustBuffer[[]TextDelta](c, value)
}

func (c FfiConverterSequenceTextDelta) Write(writer io.Writer, value []TextDelta) {
	if len(value) > math.MaxInt32 {
		panic("[]TextDelta is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterTextDeltaINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceTextDelta struct{}

func (FfiDestroyerSequenceTextDelta) Destroy(sequence []TextDelta) {
	for _, value := range sequence {
		FfiDestroyerTextDelta{}.Destroy(value)
	}
}

type FfiConverterMapUint64CounterSpan struct{}

var FfiConverterMapUint64CounterSpanINSTANCE = FfiConverterMapUint64CounterSpan{}

func (c FfiConverterMapUint64CounterSpan) Lift(rb RustBufferI) map[uint64]CounterSpan {
	return LiftFromRustBuffer[map[uint64]CounterSpan](c, rb)
}

func (_ FfiConverterMapUint64CounterSpan) Read(reader io.Reader) map[uint64]CounterSpan {
	result := make(map[uint64]CounterSpan)
	length := readInt32(reader)
	for i := int32(0); i < length; i++ {
		key := FfiConverterUint64INSTANCE.Read(reader)
		value := FfiConverterCounterSpanINSTANCE.Read(reader)
		result[key] = value
	}
	return result
}

func (c FfiConverterMapUint64CounterSpan) Lower(value map[uint64]CounterSpan) C.RustBuffer {
	return LowerIntoRustBuffer[map[uint64]CounterSpan](c, value)
}

func (_ FfiConverterMapUint64CounterSpan) Write(writer io.Writer, mapValue map[uint64]CounterSpan) {
	if len(mapValue) > math.MaxInt32 {
		panic("map[uint64]CounterSpan is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(mapValue)))
	for key, value := range mapValue {
		FfiConverterUint64INSTANCE.Write(writer, key)
		FfiConverterCounterSpanINSTANCE.Write(writer, value)
	}
}

type FfiDestroyerMapUint64CounterSpan struct{}

func (_ FfiDestroyerMapUint64CounterSpan) Destroy(mapValue map[uint64]CounterSpan) {
	for key, value := range mapValue {
		FfiDestroyerUint64{}.Destroy(key)
		FfiDestroyerCounterSpan{}.Destroy(value)
	}
}

type FfiConverterMapUint64PeerInfo struct{}

var FfiConverterMapUint64PeerInfoINSTANCE = FfiConverterMapUint64PeerInfo{}

func (c FfiConverterMapUint64PeerInfo) Lift(rb RustBufferI) map[uint64]PeerInfo {
	return LiftFromRustBuffer[map[uint64]PeerInfo](c, rb)
}

func (_ FfiConverterMapUint64PeerInfo) Read(reader io.Reader) map[uint64]PeerInfo {
	result := make(map[uint64]PeerInfo)
	length := readInt32(reader)
	for i := int32(0); i < length; i++ {
		key := FfiConverterUint64INSTANCE.Read(reader)
		value := FfiConverterPeerInfoINSTANCE.Read(reader)
		result[key] = value
	}
	return result
}

func (c FfiConverterMapUint64PeerInfo) Lower(value map[uint64]PeerInfo) C.RustBuffer {
	return LowerIntoRustBuffer[map[uint64]PeerInfo](c, value)
}

func (_ FfiConverterMapUint64PeerInfo) Write(writer io.Writer, mapValue map[uint64]PeerInfo) {
	if len(mapValue) > math.MaxInt32 {
		panic("map[uint64]PeerInfo is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(mapValue)))
	for key, value := range mapValue {
		FfiConverterUint64INSTANCE.Write(writer, key)
		FfiConverterPeerInfoINSTANCE.Write(writer, value)
	}
}

type FfiDestroyerMapUint64PeerInfo struct{}

func (_ FfiDestroyerMapUint64PeerInfo) Destroy(mapValue map[uint64]PeerInfo) {
	for key, value := range mapValue {
		FfiDestroyerUint64{}.Destroy(key)
		FfiDestroyerPeerInfo{}.Destroy(value)
	}
}

type FfiConverterMapStringLoroValue struct{}

var FfiConverterMapStringLoroValueINSTANCE = FfiConverterMapStringLoroValue{}

func (c FfiConverterMapStringLoroValue) Lift(rb RustBufferI) map[string]LoroValue {
	return LiftFromRustBuffer[map[string]LoroValue](c, rb)
}

func (_ FfiConverterMapStringLoroValue) Read(reader io.Reader) map[string]LoroValue {
	result := make(map[string]LoroValue)
	length := readInt32(reader)
	for i := int32(0); i < length; i++ {
		key := FfiConverterStringINSTANCE.Read(reader)
		value := FfiConverterLoroValueINSTANCE.Read(reader)
		result[key] = value
	}
	return result
}

func (c FfiConverterMapStringLoroValue) Lower(value map[string]LoroValue) C.RustBuffer {
	return LowerIntoRustBuffer[map[string]LoroValue](c, value)
}

func (_ FfiConverterMapStringLoroValue) Write(writer io.Writer, mapValue map[string]LoroValue) {
	if len(mapValue) > math.MaxInt32 {
		panic("map[string]LoroValue is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(mapValue)))
	for key, value := range mapValue {
		FfiConverterStringINSTANCE.Write(writer, key)
		FfiConverterLoroValueINSTANCE.Write(writer, value)
	}
}

type FfiDestroyerMapStringLoroValue struct{}

func (_ FfiDestroyerMapStringLoroValue) Destroy(mapValue map[string]LoroValue) {
	for key, value := range mapValue {
		FfiDestroyerString{}.Destroy(key)
		FfiDestroyerLoroValue{}.Destroy(value)
	}
}

type FfiConverterMapStringOptionalValueOrContainer struct{}

var FfiConverterMapStringOptionalValueOrContainerINSTANCE = FfiConverterMapStringOptionalValueOrContainer{}

func (c FfiConverterMapStringOptionalValueOrContainer) Lift(rb RustBufferI) map[string]**ValueOrContainer {
	return LiftFromRustBuffer[map[string]**ValueOrContainer](c, rb)
}

func (_ FfiConverterMapStringOptionalValueOrContainer) Read(reader io.Reader) map[string]**ValueOrContainer {
	result := make(map[string]**ValueOrContainer)
	length := readInt32(reader)
	for i := int32(0); i < length; i++ {
		key := FfiConverterStringINSTANCE.Read(reader)
		value := FfiConverterOptionalValueOrContainerINSTANCE.Read(reader)
		result[key] = value
	}
	return result
}

func (c FfiConverterMapStringOptionalValueOrContainer) Lower(value map[string]**ValueOrContainer) C.RustBuffer {
	return LowerIntoRustBuffer[map[string]**ValueOrContainer](c, value)
}

func (_ FfiConverterMapStringOptionalValueOrContainer) Write(writer io.Writer, mapValue map[string]**ValueOrContainer) {
	if len(mapValue) > math.MaxInt32 {
		panic("map[string]**ValueOrContainer is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(mapValue)))
	for key, value := range mapValue {
		FfiConverterStringINSTANCE.Write(writer, key)
		FfiConverterOptionalValueOrContainerINSTANCE.Write(writer, value)
	}
}

type FfiDestroyerMapStringOptionalValueOrContainer struct{}

func (_ FfiDestroyerMapStringOptionalValueOrContainer) Destroy(mapValue map[string]**ValueOrContainer) {
	for key, value := range mapValue {
		FfiDestroyerString{}.Destroy(key)
		FfiDestroyerOptionalValueOrContainer{}.Destroy(value)
	}
}

// Decodes the metadata for an imported blob from the provided bytes.
func DecodeImportBlobMeta(bytes []byte, checkChecksum bool) (ImportBlobMetadata, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LoroError](FfiConverterLoroError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_func_decode_import_blob_meta(FfiConverterBytesINSTANCE.Lower(bytes), FfiConverterBoolINSTANCE.Lower(checkChecksum), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue ImportBlobMetadata
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterImportBlobMetadataINSTANCE.Lift(_uniffiRV), nil
	}
}

func GetVersion() string {
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_loro_fn_func_get_version(_uniffiStatus),
		}
	}))
}
