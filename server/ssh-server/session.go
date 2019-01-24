package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"

	sshserver "github.com/gliderlabs/ssh"
	"golang.org/x/crypto/ssh"
)

var ErrInvalidSessionTarget = errors.New("Invalid session target")

type Session struct {
	session sshserver.Session
	user    string
	target  string
	port    uint32
}

func NewSession(target string, session sshserver.Session) (*Session, error) {
	s := &Session{session: session}

	parts := strings.SplitN(target, "@", 2)
	if len(parts) != 2 {
		return nil, ErrInvalidSessionTarget
	}

	s.user = parts[0]
	s.target = parts[1]

	return s, nil
}

// connect connects to reverse tunnel port
func (s *Session) connect(passwd string) error {
	config := &ssh.ClientConfig{
		User: s.user,
		Auth: []ssh.AuthMethod{
			ssh.Password(passwd),
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}

	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", "localhost", s.port), config)
	if err != nil {
		return err
	}

	client, err := conn.NewSession()
	if err != nil {
		fmt.Println(err)
	}

	pty, winCh, isPty := s.session.Pty()

	if isPty {
		err = client.RequestPty(pty.Term, pty.Window.Height, pty.Window.Width, ssh.TerminalModes{})
		if err != nil {
			return err
		}

		go func() {
			for win := range winCh {
				if err = client.WindowChange(win.Height, win.Width); err != nil {
					fmt.Println(err)
				}
			}
		}()

		stdin, err := client.StdinPipe()
		if err != nil {
			return err
		}
		stdout, err := client.StdoutPipe()
		if err != nil {
			return err
		}

		go func() {
			if _, err = io.Copy(stdin, s.session); err != nil {
				fmt.Println(err)
			}
		}()

		go func() {
			if _, err = io.Copy(s.session, stdout); err != nil {
				fmt.Println(err)
			}
		}()

		if err = client.Shell(); err != nil {
			return err
		}

		if err = client.Wait(); err != nil {
			return err
		}
	}

	return nil
}

// promptForPassword prompts the user for password of session target
func (s *Session) promptForPassword() (string, error) {
	io.WriteString(s.session, "password: ")

	reader := bufio.NewReader(s.session)
	passwd, err := reader.ReadString('\r')
	if err != nil {
		return "", err
	}

	passwd = strings.TrimSpace(passwd)

	io.WriteString(s.session, "\n")

	return passwd, nil
}
