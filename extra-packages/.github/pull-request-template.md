**CVE Scanning:** This PR will fail if ANY CVEs are found (fail-any mode). To customize:
- **Must-fix specific CVEs only:** Add `<!--ci-cve-scan:must-fix: CVE-ID-->` markers and remove the line below
- **Fail on any CVEs (default):** Keep the marker below

```
<!--ci-cve-scan:fail-any-->
```