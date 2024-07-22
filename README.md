# GoMailer
 CLI email spoof tool inspired by Kali Linux's 'sendemail'

 ![Alt text](/gomail.jpg)
 

# GoMailer Sender

A simple Go script for sending emails with attachments and inline images. 

## Prerequisites

- Go 1.18 or higher
- SMTP server credentials

## Installation 

Clone the repository:
   ```sh
   git clone https://github.com/yourusername/go-email-sender.git
   cd go-email-sender
   Install dependencies: 
   sh

go mod tidy

Create a config.ini file with your SMTP server credentials. The file should include: 

[SMPT]
server=smtp.example.com
port=587
username=your-username
password=your-password

Build and run the script:

sh
    go run gomail.go

Usage
Follow the prompts to enter your details, recipient’s email, subject, body, and optional attachments.
