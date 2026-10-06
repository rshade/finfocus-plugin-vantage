# FOCUS 1.4 field availability

The Vantage cost response exposes accrued date, amount, currency, provider,
service, account, region, resource ID, and tags. It does not expose invoice line
identifiers or commitment program eligibility details. The plugin therefore
leaves `invoice_detail_id` and `commitment_program_eligibility_details` unset.

A FOCUS record is attached when the source supplies the account, service,
currency, and interpretable positive usage with a unit needed by the SDK's
validator. The caller's `billing_account_id` takes precedence over source
account IDs. If source fields are insufficient, the optional record is omitted;
the plugin does not invent usage, invoice IDs, or commitments.

No feature toggle is needed for unavailable fields. When Vantage exposes these
fields, extend the wrapper and map them through
`pluginsdk.FocusRecordBuilder.WithInvoiceDetailID` and
`WithCommitmentProgramEligibilityDetails`, then validate the completed record.
