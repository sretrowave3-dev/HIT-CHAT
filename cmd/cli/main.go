package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/hit-chat/campusnet/internal/crypto"
)

func main() {
	serverURL := "http://localhost:8080"
	if envURL := os.Getenv("CAMPUSNET_SERVER"); envURL != "" {
		serverURL = envURL
	}

	if len(os.Args) < 2 {
		// Default: launch interactive TUI mode
		launchInteractive(serverURL)
		return
	}

	subcommand := os.Args[1]
	switch subcommand {
	case "pair":
		pairCmd := flag.NewFlagSet("pair", flag.ExitOnError)
		srv := pairCmd.String("server", serverURL, "Server gateway URL")
		handle := pairCmd.String("user", "campus_dev", "User handle to link")
		_ = pairCmd.Parse(os.Args[2:])

		if err := RunPair(*srv, *handle); err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Device pairing failed: %v\n", err)
			os.Exit(1)
		}

	case "send":
		sendCmd := flag.NewFlagSet("send", flag.ExitOnError)
		to := sendCmd.String("to", "", "Recipient user handle (e.g. @alice)")
		ttl := sendCmd.Int64("ttl", 10000, "Millisecond TTL before automatic message burn")
		srv := sendCmd.String("server", serverURL, "Server gateway URL")
		_ = sendCmd.Parse(os.Args[2:])

		if *to == "" {
			fmt.Fprintln(os.Stderr, "Error: --to recipient flag is required (e.g. --to @alice)")
			os.Exit(1)
		}

		// Read message body either from positional argument or standard input pipe
		message := strings.Join(sendCmd.Args(), " ")
		if message == "" {
			piped, err := ReadStdinMessage()
			if err != nil || piped == "" {
				fmt.Fprintln(os.Stderr, "Error: message payload is required as argument or stdin pipe")
				os.Exit(1)
			}
			message = piped
		}

		if err := RunSend(*srv, *to, message, *ttl); err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Send failed: %v\n", err)
			os.Exit(1)
		}

	case "listen":
		listenCmd := flag.NewFlagSet("listen", flag.ExitOnError)
		format := listenCmd.String("format", "text", "Output format: text or json")
		autoBurn := listenCmd.Bool("burn", true, "Automatically acknowledge and burn messages on read")
		srv := listenCmd.String("server", serverURL, "Server gateway URL")
		_ = listenCmd.Parse(os.Args[2:])

		if err := RunListen(*srv, *format, *autoBurn); err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Listen failed: %v\n", err)
			os.Exit(1)
		}

	case "tui":
		launchInteractive(serverURL)

	case "help", "--help", "-h":
		printHelp()

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", subcommand)
		printHelp()
		os.Exit(1)
	}
}

func launchInteractive(serverURL string) {
	secStore, err := crypto.NewSecureStore()
	userHandle := "student_guest"
	deviceID := "dev-cli-local"

	if err == nil {
		if h, err := secStore.Get("user_handle"); err == nil && len(h) > 0 {
			userHandle = string(h)
		}
		if d, err := secStore.Get("device_id"); err == nil && len(d) > 0 {
			deviceID = string(d)
		}
	}

	if err := RunTUI(serverURL, userHandle, deviceID); err != nil {
		fmt.Fprintf(os.Stderr, "[TUI ERROR] %v\n", err)
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println(`
CampusNet CLI - High-Security Ephemeral Communications

USAGE:
  campusnet [command] [flags]

COMMANDS:
  tui                  Launch interactive split-screen Terminal UI (default)
  pair                 Initiate RFC 8628 terminal device authorization
  send --to @user      Send encrypted ephemeral message with burn timer
  listen               Stream incoming decrypted messages from gateway
  help                 Display help options

EXAMPLES:
  # Launch interactive TUI
  campusnet

  # Pair device with web portal
  campusnet pair --server http://localhost:8080

  # Send ephemeral message with 5s burn timer
  campusnet send --to @alice --ttl 5000 "Meet at Robotics Lab 4"

  # Send message via POSIX stdin pipe
  echo "Confidential coordinate" | campusnet send --to @bob

  # Stream incoming messages in JSON format
  campusnet listen --format json
`)
}
