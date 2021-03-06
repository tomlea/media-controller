package server

import "net"
import "github.com/cwninja/media-controller/tv"
import "github.com/yasuyuky/jsonpath"
import "bufio"
import "time"
import "bytes"
import jsonEncoding "encoding/json"


type Server struct {
  Listener * net.TCPListener
  TV * tv.TV
}

func New(tv * tv.TV, listenAddress string) (* Server, error) {
  tcpAddress, err := net.ResolveTCPAddr("tcp", listenAddress)
  if err != nil {
    return nil, err
  }

  listener, err := net.ListenTCP("tcp", tcpAddress)
  if err != nil {
    return nil, err
  }

  server := Server{ Listener: listener, TV: tv }
  return &server, nil
}

func (s * Server) Start() {
  for {
    if c, err := s.Listener.AcceptTCP(); err == nil {
      go s.HandleConnection(c)
    }
  }
}

func (s * Server) HandleConnection(c * net.TCPConn) {
  defer c.Close()
  messageScanner := bufio.NewScanner(c)
  for messageScanner.Scan() {
    if json, err := jsonpath.DecodeString(messageScanner.Text()); err == nil {
      command, _ := jsonpath.GetString(json, []interface{}{"command"}, "")
      if command == "players" {
        c.Write(bytes.NewBufferString("[\"default\"]\n").Bytes())
      } else if command == "stop" || command == "exit" {
        s.TV.Stop()
      } else if command == "pause" {
        s.TV.Pause()
      } else if command == "play" {
        url, _ := jsonpath.GetString(json, []interface{}{"url"}, "")
        position, _ := jsonpath.GetNumber(json, []interface{}{"position"}, 0)
        s.TV.PlayFrom(url, int(position))
      } else if command == "status" {
        data, _ := jsonEncoding.Marshal(s.TV.Status())
        c.Write(data)
        c.Write([]byte{'\n'})
      } else if command == "seek_to" {
        position, _ := jsonpath.GetNumber(json, []interface{}{"position"}, 0)
        s.TV.SeekTo(int(position))
      } else if command == "seek_by" {
        seconds, _ := jsonpath.GetNumber(json, []interface{}{"seconds"}, 0)
        s.TV.SeekBy(int(seconds))
      } else if command == "monitor" || command == "subscribe" {
        lastStatus := tv.Status{Url: "none"}
        for {
          newStatus := s.TV.Status()
          if newStatus != lastStatus {
            lastStatus = newStatus
            data, _ := jsonEncoding.Marshal(lastStatus)
            if _, err := c.Write(data); err != nil {
              return
            }
            if _, err := c.Write([]byte{'\n'}); err != nil {
              return
            }
          }
          time.Sleep(time.Second)
        }
      }
    }
  }
}
