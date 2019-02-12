package pfs

import (
	"bytes"
	"context"
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

func NewStreamingGFRReader(client StreamingGFRClient, cancel context.CancelFunc) io.ReadCloser {
	return &streamingGFRReader{client: client, cancel: cancel}
}

type streamingGFRReader struct {
	client StreamingGFRClient
	buffer bytes.Buffer
	cancel context.CancelFunc
}

func(s *streamingGFRReader) Read(p []byte) (int, error) {
	// TODO this is doing an unneeded copy (unless go is smarter than I think it is)
	if s.buffer.Len() == 0 {
		gfr, err := s.client.Recv()
		if err != nil {
			return 0, err
		}
		s.buffer.Reset()
		b, err := proto.Marshal(gfr)
		if err != nil {
			return 0, err
		}
		if _, err := s.buffer.Write(b); err != nil {
			return 0, err
		}
	}
	return s.buffer.Read(p)
}

func(s *streamingGFRReader) Close() error {
	if s.cancel != nil {
		s.cancel()
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