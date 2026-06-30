
import (
	"fmt"
	"net"
	"os"
	"strings"
)

type DomainPosture struct {
	Domain      string
	SPFRecord   string
	SPFFound    bool
	DKIMHints   []string
	DMARCRecord string
	DMARCFound  bool
	DMARCPolicy string
}

var commonDKIMSelectors = []string{
	"default", "selector1", "selector2", "google", "k1", "dkim", "mail",
	"smtp", "brevo", "mandrill", "sendgrid", "s1", "s2",
}

func checkDomainPosture(domain string) DomainPosture {
	p := DomainPosture{Domain: domain}

	txtRecords, _ := net.LookupTXT(domain)
	for _, r := range txtRecords {
		if strings.HasPrefix(strings.ToLower(r), "v=spf1") {
			p.SPFRecord = r
			p.SPFFound = true
			break
		}
	}

	dmarcRecords, _ := net.LookupTXT("_dmarc." + domain)
	for _, r := range dmarcRecords {
		if strings.HasPrefix(strings.ToLower(r), "v=dmarc1") {
			p.DMARCRecord = r
			p.DMARCFound = true
			p.DMARCPolicy = extractTag(r, "p")
			break
		}
	}

	for _, sel := range commonDKIMSelectors {
		name := fmt.Sprintf("%s._domainkey.%s", sel, domain)
		recs, err := net.LookupTXT(name)
		if err == nil && len(recs) > 0 {
			p.DKIMHints = append(p.DKIMHints, sel)
		}
	}

	return p
}

func extractTag(record, tag string) string {
	parts := strings.Split(record, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, tag+"=") {
			return strings.TrimPrefix(part, tag+"=")
		}
	}
	return ""
}

func (p DomainPosture) verdict() string {
	if !p.DMARCFound {
		return "NO DMARC RECORD — domain has no spoofing protection. Exact-domain " +
			"forgery in the From: header will likely reach the inbox on receivers " +
			"that don't enforce sender authentication independently."
	}
	switch p.DMARCPolicy {
	case "reject":
		return "DMARC p=reject — exact-domain spoofing will be rejected by any " +
			"enforcing receiver (Gmail, Outlook, ProtonMail, etc). Direct From: " +
			"forgery of this domain is not viable; consider cousin-domain or " +
			"display-name approaches instead."
	case "quarantine":
		return "DMARC p=quarantine — exact-domain spoofs will typically be junked " +
			"rather than delivered to inbox on enforcing receivers."
	case "none":
		return "DMARC p=none (monitor-only) — the domain publishes a policy but " +
			"isn't enforcing it. Exact-domain spoofing may still reach the inbox " +
			"depending on the receiving provider's own heuristics."
	default:
		return "DMARC record present but policy tag unclear — manual review needed."
	}
}

func printPosture(p DomainPosture) {
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Domain authentication posture: %s\n", p.Domain)
	fmt.Println(strings.Repeat("=", 60))

	fmt.Println("\n[SPF]")
	if p.SPFFound {
		fmt.Println(p.SPFRecord)
	} else {
		fmt.Println("No SPF record found.")
	}

	fmt.Println("\n[DKIM] (best-effort common selector probe)")
	if len(p.DKIMHints) > 0 {
		fmt.Printf("Selectors found: %s\n", strings.Join(p.DKIMHints, ", "))
	} else {
		fmt.Println("No common selectors resolved. Check a sample message's headers if available.")
	}

	fmt.Println("\n[DMARC]")
	if p.DMARCFound {
		fmt.Println(p.DMARCRecord)
		fmt.Printf("Policy: p=%s\n", p.DMARCPolicy)
	} else {
		fmt.Println("No DMARC record found.")
	}

	fmt.Println("\n[Verdict]")
	fmt.Println(p.verdict())
	fmt.Println()
}

func runCheckMode() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: gomailer --check <domain>")
		os.Exit(1)
	}
	domain := strings.TrimSpace(os.Args[2])
	posture := checkDomainPosture(domain)
	printPosture(posture)
}
