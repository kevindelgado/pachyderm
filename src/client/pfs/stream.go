package pfs

import (
	"bytes"
	"context"
	"fmt"
	"github.com/pachyderm/pachyderm/src/client/pkg/grpcutil"
	"io"

	"github.com/golang/protobuf/proto"
)

type StreamingGFRServer interface {
	Send(gfr *GetFileResponse) error
}

type StreamingGFRClient interface {
	Recv() (*GetFileResponse, error)
}

func NewStreamingGFRReader(buffer bytes.Buffer, cancel context.CancelFunc) io.ReadCloser {
	return &StreamingGFRReader{Buffer: buffer, Cancel: cancel}
}

type StreamingGFRReader struct {
	//client StreamingGFRClient
	Buffer bytes.Buffer
	Cancel context.CancelFunc
}

func(s *StreamingGFRReader) Read(p []byte) (int, error) {
	// TODO this is doing an unneeded copy (unless go is smarter than I think it is)
	if s.Buffer.Len() == 0 {
		fmt.Println("streaming gfr has empty buffer")
		//gfr, err := s.client.Recv()
		//if err != nil {
		//	return 0, err
		//}
		//s.buffer.Reset()
		//b, err := proto.Marshal(gfr)
		//if err != nil {
		//	return 0, err
		//}
		//if _, err := s.buffer.Write(b); err != nil {
		//	return 0, err
		//}
	}
	return s.Buffer.Read(p)
}

func(s *StreamingGFRReader) Close() error {
	if s.Cancel != nil {
		s.Cancel()
	}
	return nil
}

///////////// Writer stuff
func NewStreamingGFRWriter(server StreamingGFRServer) io.Writer {
	return &streamingGFRWriter{server}
}

type streamingGFRWriter struct {
	server StreamingGFRServer
}

func (s *streamingGFRWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	var gfr *GetFileResponse
	if err := proto.Unmarshal(p, gfr); err != nil {
		return 0, err
	}
	if err := s.server.Send(gfr); err != nil {
		return 0, err
	}
	return len(p), nil
}

func WriteToStreamingGFRServer(reader io.Reader, server StreamingGFRServer) error {
	buf := grpcutil.GetBuffer()
	defer grpcutil.PutBuffer(buf)
	_, err := io.CopyBuffer(NewStreamingGFRWriter(server), grpcutil.ReaderWrapper{reader}, buf)
	return err
}

func WriteFromStreamingGFRClient(client StreamingGFRClient, writer io.Writer) error {
	for gfr, err := client.Recv(); err != io.EOF; gfr, err = client.Recv() {
		if err != nil {
			return err
		}
		b, err := proto.Marshal(gfr)
		if err != nil {
			return err
		}
		if _, err = writer.Write(b); err != nil {
			return err
		}
	}
	return nil
}