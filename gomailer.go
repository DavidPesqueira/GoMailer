
package main

import (
    "bufio"
    "bytes"
    "encoding/base64"
    "fmt"
    "log"
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

██╗   ██╗███████╗██████╗      ██╗    ██████╗

██║   ██║██╔════╝██╔══██╗    ███║   ██╔═████╗

██║   ██║█████╗  ██████╔╝    ╚██║   ██║██╔██║

╚██╗ ██╔╝██╔══╝  ██╔══██╗     ██║   ████╔╝██║

╚████╔╝ ███████╗██║  ██║     ██║██╗╚██████╔╝

  ╚═══╝  ╚══════╝╚═╝  ╚═╝     ╚═╝╚═╝ ╚═════╝

`

func completer(d prompt.Document) []prompt.Suggest {
    // List of suggestions
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

// Read SMTP configuration from a file
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
        if strings.TrimSpace(line) == "" {
            continue
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
    buffer.WriteString(fmt.Sprintf("To: %s <%s>\r\n", fromName, toEmail))
    if ccEmails != "" {
        ccNamesEmails := strings.Split(ccEmails, ",")
        ccNamesList := strings.Split(ccNames, ",")
        for i := range ccNamesEmails {
            buffer.WriteString(fmt.Sprintf("Cc: %s <%s>\r\n", ccNamesList[i], ccNamesEmails[i]))
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

func main() {
    // Print ASCII Art
    fmt.Println(asciiArt)
    fmt.Println(headerArt)

    // Read SMTP configuration
    config, err := readConfig("config.ini")
    if err != nil {
        log.Fatalf("Error reading config file: %v", err)
    }
    server := config["server"]
    port := config["port"]
    username := config["username"]
    password := config["password"]

    // Show current working directory
    dir, err := os.Getwd()
    if err != nil {
        log.Fatalf("Error getting current directory: %v", err)
    }
    fmt.Printf("Current working directory: %s\n", dir)

    // Prompt for email details
    reader := bufio.NewReader(os.Stdin)
    fmt.Print("Enter your name: ")
    fromName, _ := reader.ReadString('\n')
    fromName = strings.TrimSpace(fromName)
    fmt.Print("Enter your email address: ")
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
        fmt.Print("Enter CC names and emails (one per line, empty line to finish):\n")
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

    // Create email message
    emailMessage := createEmail(fromName, fromEmail, toEmail, ccNames, ccEmails, subject, body, imageBase64, attachmentBase64, attachmentName)

    // Send email
    auth := smtp.PlainAuth("", username, password, server)
    err = smtp.SendMail(server+":"+port, auth, fromEmail, []string{toEmail}, []byte(emailMessage))
    if err != nil {
        log.Fatalf("Error sending email: %v", err)
    }
    fmt.Println("Email sent successfully!")
}


