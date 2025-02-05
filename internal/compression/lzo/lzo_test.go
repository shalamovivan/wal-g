//go:build lzo
// +build lzo

package lzo_test

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"testing"

	"github.com/cyberdelia/lzo"
	"github.com/stretchr/testify/assert"
	"github.com/wal-g/wal-g/internal"
	walg_lzo "github.com/wal-g/wal-g/internal/compression/lzo"
	"github.com/wal-g/wal-g/testtools"
	"github.com/wal-g/wal-g/utility"
)

// Test extraction of various lzo compressed tar files.
func testLzopRoundTrip(t *testing.T, stride, nBytes int) {
	// Generate and save random bytes compare against
	// compression-decompression cycle.
	sb := testtools.NewStrideByteReader(stride)
	lr := &io.LimitedReader{
		R: sb,
		N: int64(nBytes),
	}
	b, err := ioutil.ReadAll(lr)

	// Copy generated bytes to another slice to make the test more
	// robust against modifications of "b".
	bCopy := make([]byte, len(b))
	copy(bCopy, b)
	if err != nil {
		t.Log(err)
	}

	// Compress bytes and make a tar in memory.
	r, w := io.Pipe()
	lzow := lzo.NewWriter(w)
	go func() {
		defer utility.LoggedClose(lzow, "")
		defer utility.LoggedClose(w, "")
		bw := bufio.NewWriterSize(lzow, walg_lzo.LzopBlockSize)
		defer func() {
			if err := bw.Flush(); err != nil {
				panic(err)
			}
		}()

		testtools.CreateTar(bw, &io.LimitedReader{
			R: bytes.NewBuffer(b),
			N: int64(len(b)),
		})

	}()
	tarContents := &bytes.Buffer{}
	io.Copy(tarContents, r)

	// Extract the generated tar and check that its one member is
	// the same as the bytes generated to begin with.
	brm := &BufferReaderMaker{tarContents, "/usr/local.lzo"}
	buf := &testtools.BufferTarInterpreter{}
	files := []internal.ReaderMaker{brm}
	err = internal.ExtractAll(buf, files)
	if err != nil {
		t.Log(err)
	}

	assert.Equalf(t, bCopy, buf.Out, "extract: Decompressed output does not match input.")
}

func TestLzopUncompressableBytes(t *testing.T) {
	testLzopRoundTrip(t, walg_lzo.LzopBlockSize*2, walg_lzo.LzopBlockSize*2)
}
func TestLzop1Byte(t *testing.T)   { testLzopRoundTrip(t, 7924, 1) }
func TestLzop1MByte(t *testing.T)  { testLzopRoundTrip(t, 7924, 1024*1024) }
func TestLzop10MByte(t *testing.T) { testLzopRoundTrip(t, 7924, 10*1024*1024) }

func setupRand(stride, nBytes int) *BufferReaderMaker {
	sb := testtools.NewStrideByteReader(stride)
	lr := &io.LimitedReader{
		R: sb,
		N: int64(nBytes),
	}
	b := &BufferReaderMaker{&bytes.Buffer{}, "/usr/local.lzo"}

	pr, pw := io.Pipe()
	lzow := lzo.NewWriter(pw)

	go func() {
		testtools.CreateTar(lzow, lr)
		defer utility.LoggedClose(lzow, "")
		defer utility.LoggedClose(pw, "")
	}()

	_, err := io.Copy(b.Buf, pr)

	if err != nil {
		panic(err)
	}

	return b
}

func BenchmarkExtractAll(b *testing.B) {
	b.SetBytes(int64(b.N * 1024 * 1024))
	out := make([]internal.ReaderMaker, 1)
	rand := setupRand(7924, b.N*1024*1024)
	fmt.Println("B.N", b.N)

	out[0] = rand

	b.ResetTimer()

	// f := &extract.FileTarInterpreter{
	// 		DBDataDirectory: "",
	// 	}
	// out[0] = f

	// extract.ExtractAll(f, out)

	// np := &extract.NOPTarInterpreter{}
	// extract.ExtractAll(np, out)

	buf := &testtools.BufferTarInterpreter{}
	err := internal.ExtractAll(buf, out)
	if err != nil {
		b.Log(err)
	}

}

// Used to mock files in memory.
type BufferReaderMaker struct {
	Buf *bytes.Buffer
	Key string
}

func (b *BufferReaderMaker) Reader() (io.ReadCloser, error) { return ioutil.NopCloser(b.Buf), nil }
func (b *BufferReaderMaker) Path() string                   { return b.Key }
func (b *BufferReaderMaker) FileType() internal.FileType    { return internal.TarFileType }
func (b *BufferReaderMaker) Mode() int                      { return 0 }

func init() {
	internal.ConfigureSettings("")
	internal.InitConfig()
	internal.Configure()
}
