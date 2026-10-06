# FOCUS 1.4 field availability

The Vantage cost response wrapper currently exposes accrued date, amount,
currency, provider, service, account, region, resource ID, and tags. It does
not expose invoice line identifiers or commitment program eligibility details.
The adapter therefore does not populate `invoice_detail_id` or
`commitment_program_eligibility_details`.

The v0.7.0 `GetActualCostResponse` still contains the FOCUS cost record as an
optional field. The plugin currently leaves that field unset because a partial
record would not satisfy the SDK's FOCUS validation requirements. When Vantage
adds invoice or commitment metadata to its API response, map it through
`pluginsdk.FocusRecordBuilder.WithInvoiceDetailID` and
`WithCommitmentProgramEligibilityDetails`, then validate the completed record
before attaching it. No feature flag is needed while the source fields are
unavailable.
