package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/hit-chat/campusnet/internal/crypto"
)

func main() {
	// Parse global --as flag for dev identity scoping
	var filteredArgs []string
	for i := 0; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "--as" && i+1 < len(os.Args) {
			_ = os.Setenv("CAMPUSNET_AS", os.Args[i+1])
			i++
		} else if strings.HasPrefix(arg, "--as=") {
			_ = os.Setenv("CAMPUSNET_AS", strings.TrimPrefix(arg, "--as="))
		} else {
			filteredArgs = append(filteredArgs, arg)
		}
	}
	os.Args = filteredArgs

	serverURL := "http://localhost:8080"
	if envURL := os.Getenv("CAMPUSNET_SERVER"); envURL != "" {
		serverURL = envURL
	}

	if len(os.Args) < 2 {
		// Default: launch interactive TUI mode with auto-connect
		launchInteractive(serverURL, "", true)
		return
	}

	subcommand := os.Args[1]
	switch subcommand {
	case "whoami":
		secStore, err := crypto.NewSecureStore()
		if err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Opening secure store: %v\n", err)
			os.Exit(1)
		}
		handle, _ := secStore.Get("user_handle")
		deviceID, _ := secStore.Get("device_id")
		asUser := os.Getenv("CAMPUSNET_AS")

		activeHandle := string(handle)
		if activeHandle == "" {
			if asUser != "" {
				activeHandle = asUser
			} else {
				activeHandle = "campus_dev (unregistered)"
			}
		}

		fmt.Println("==================================================================")
		fmt.Println("             CAMPUSNET CLIENT IDENTITY INSPECTOR                  ")
		fmt.Println("==================================================================")
		fmt.Printf(" Active User Handle:  @%s\n", strings.TrimPrefix(activeHandle, "@"))
		if len(deviceID) > 0 {
			fmt.Printf(" Active Device ID:    %s\n", string(deviceID))
		} else {
			fmt.Println(" Active Device ID:    (ephemeral dev session)")
		}
		if asUser != "" {
			fmt.Printf(" Scoped Via:          --as %s\n", asUser)
		}
		fmt.Println(" Cryptographic Trust: Zero-Knowledge Pairwise HKDF / Double Ratchet")
		fmt.Println("==================================================================")
		return

	case "pair":
		pairCmd := flag.NewFlagSet("pair", flag.ExitOnError)
		srv := pairCmd.String("server", serverURL, "Server gateway URL")
		handle := pairCmd.String("user", "campus_dev", "User handle to link")
		auto := pairCmd.Bool("auto", false, "Automatically authorize without manual web verification")
		_ = pairCmd.Parse(os.Args[2:])

		if *auto {
			if err := RunAutoPair(*srv, *handle); err != nil {
				fmt.Fprintf(os.Stderr, "[ERROR] Auto-pairing failed: %v\n", err)
				os.Exit(1)
			}
			return
		}

		if err := RunPair(*srv, *handle); err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Device pairing failed: %v\n", err)
			os.Exit(1)
		}

	case "connect":
		connectCmd := flag.NewFlagSet("connect", flag.ExitOnError)
		srv := connectCmd.String("server", serverURL, "Server gateway URL")
		handle := connectCmd.String("user", "campus_dev", "User handle to link")
		auto := connectCmd.Bool("auto", true, "Auto-authorize and establish live connection")
		_ = connectCmd.Parse(os.Args[2:])

		targetHandle := *handle
		if strings.HasPrefix(targetHandle, "-") {
			targetHandle = "campus_dev"
		}
		// Accept positional handle e.g. "campusnet connect alice"
		for _, arg := range connectCmd.Args() {
			if !strings.HasPrefix(arg, "-") && arg != "" {
				targetHandle = arg
				break
			}
		}

		if err := RunConnect(*srv, targetHandle, *auto); err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Connect failed: %v\n", err)
			os.Exit(1)
		}

	case "send":
		sendCmd := flag.NewFlagSet("send", flag.ExitOnError)
		to := sendCmd.String("to", "", "Recipient user handle (e.g. @alice)")
		from := sendCmd.String("user", "", "Sender user handle (alias: --from)")
		fromAlt := sendCmd.String("from", "", "Sender user handle")
		ttl := sendCmd.Int64("ttl", 10000, "Millisecond TTL before automatic message burn")
		srv := sendCmd.String("server", serverURL, "Server gateway URL")
		_ = sendCmd.Parse(os.Args[2:])

		sender := *from
		if sender == "" && *fromAlt != "" {
			sender = *fromAlt
		}
		if strings.HasPrefix(sender, "-") {
			sender = ""
		}

		args := sendCmd.Args()
		if *to == "" && len(args) > 0 {
			if strings.HasPrefix(args[0], "@") || (!strings.Contains(args[0], " ") && len(args) > 1) {
				*to = args[0]
				args = args[1:]
			}
		}

		if *to == "" {
			fmt.Fprintln(os.Stderr, "Error: --to recipient flag is required (e.g. --to @alice)")
			os.Exit(1)
		}

		// Read message body either from positional argument or standard input pipe
		message := strings.Join(args, " ")
		if message == "" {
			piped, err := ReadStdinMessage()
			if err != nil || piped == "" {
				fmt.Fprintln(os.Stderr, "Error: message payload is required as argument or stdin pipe")
				os.Exit(1)
			}
			message = piped
		}

		if err := RunSend(*srv, sender, *to, message, *ttl); err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Send failed: %v\n", err)
			os.Exit(1)
		}

	case "listen":
		listenCmd := flag.NewFlagSet("listen", flag.ExitOnError)
		user := listenCmd.String("user", "", "User handle to listen as (e.g. @alice)")
		format := listenCmd.String("format", "text", "Output format: text or json")
		autoBurn := listenCmd.Bool("burn", true, "Automatically acknowledge and burn messages on read")
		srv := listenCmd.String("server", serverURL, "Server gateway URL")
		_ = listenCmd.Parse(os.Args[2:])

		if err := RunListen(*srv, *user, *format, *autoBurn); err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Listen failed: %v\n", err)
			os.Exit(1)
		}

	case "tui":
		tuiCmd := flag.NewFlagSet("tui", flag.ExitOnError)
		srv := tuiCmd.String("server", serverURL, "Server gateway URL")
		handle := tuiCmd.String("user", "", "User handle (defaults to stored handle or student_guest)")
		auto := tuiCmd.Bool("auto", true, "Auto-authorize and connect WebSocket on launch")
		_ = tuiCmd.Parse(os.Args[2:])
		launchInteractive(*srv, *handle, *auto)

	case "version", "--version", "-v":
		fmt.Println("CampusNet CLI v1.0.0 (High-Security E2EE Ephemeral Network)")
		fmt.Println("Engine: Curve25519 X3DH + Double Ratchet + Redis/RAM Ephemerality")
		return

	case "help", "--help", "-h":
		printHelp()

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", subcommand)
		printHelp()
		os.Exit(1)
	}
}

func launchInteractive(serverURL, userHandleOverride string, autoConnect bool) {
	secStore, err := crypto.NewSecureStore()
	userHandle := "student_guest"
	deviceID := "dev-cli-local"
	token := ""

	if userHandleOverride != "" {
		userHandle = strings.TrimPrefix(strings.ToLower(userHandleOverride), "@")
	}

	if err == nil {
		if h, err := secStore.Get("user_handle"); err == nil && len(h) > 0 && userHandleOverride == "" {
			userHandle = string(h)
		}
		if d, err := secStore.Get("device_id"); err == nil && len(d) > 0 {
			deviceID = string(d)
		}
		if t, err := secStore.Get("access_token"); err == nil && len(t) > 0 {
			token = string(t)
		}
	}

	// If autoConnect is requested and no token exists (or user override supplied), auto-authorize
	if autoConnect && token == "" {
		if t, d, h, aErr := AutoAuthorizeAndRegister(serverURL, userHandle); aErr == nil {
			token = t
			deviceID = d
			userHandle = h
		}
	}

	if err := RunTUI(serverURL, userHandle, deviceID, token); err != nil {
		fmt.Fprintf(os.Stderr, "[TUI ERROR] %v\n", err)
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Print(`
CampusNet CLI - High-Security Ephemeral Communications

USAGE:
  campusnet [command] [flags]

COMMANDS:
  tui                  Launch interactive split-screen Terminal UI (default, auto-connects)
  connect              Auto-authorize and establish live connection to gateway
  pair                 Initiate RFC 8628 terminal device authorization (--auto for instant)
  send --to @user      Send encrypted ephemeral message with burn timer (--user/--from)
  listen               Stream incoming decrypted messages from gateway (--user)
  help                 Display help options

EXAMPLES:
  # Launch interactive TUI (auto-connects automatically)
  campusnet

  # Auto-connect and verify live gateway session
  campusnet connect --user @student_guest

  # Auto-authorize device instantly without manual web verification
  campusnet pair --auto --user @campus_dev

  # Send ephemeral message as @bob to @alice with 5s burn timer
  campusnet send --from @bob --to @alice --ttl 5000 "Meet at Robotics Lab 4"

  # Send message via POSIX stdin pipe
  echo "Confidential coordinate" | campusnet send --from @bob --to @alice

  # Stream incoming messages in JSON format as @alice
  campusnet listen --user @alice --format json
`)
}
