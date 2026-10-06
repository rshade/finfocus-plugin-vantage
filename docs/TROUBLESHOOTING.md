# Troubleshooting Guide

This guide helps you diagnose and resolve common issues with the FinFocus
Vantage adapter.

## Common Issues & Solutions

### Issue 1: Authentication Failed (401 Unauthorized)

**Symptoms:**

- Error: `401 Unauthorized` or `invalid token`
- API requests fail with authentication errors
- Error messages mentioning invalid or expired tokens

**Causes:**

- API token missing or not set in environment
- Token expired or revoked
- Token lacks required permissions
- Wrong token type used

**Solutions:**

1. **Verify token is set**:

   ```bash
   echo $FINFOCUS_VANTAGE_TOKEN
   # Should output: vantage_... or cr_... or ws_... (not empty)
   ```

2. **Test token validity**:

   ```bash
   # Test with simple API call
   curl -H "Authorization: Bearer $FINFOCUS_VANTAGE_TOKEN" \
     https://api.vantage.sh/costs
   # Should return 200 or 400 (bad params), NOT 401
   ```

3. **Regenerate expired tokens**:
   - Log into Vantage console
   - Generate new API token
   - Update `FINFOCUS_VANTAGE_TOKEN` environment variable

4. **Verify token type and scope**:
   - Cost Report tokens (`cr_*`): Scoped to specific reports (preferred)
   - Workspace tokens (`ws_*`): Broader workspace access
   - Ensure token has read access to cost data

5. **Configuration verification**:

   ```yaml
   credentials:
     token: ${FINFOCUS_VANTAGE_TOKEN}  # Use env var, not hardcoded
   params:
     cost_report_token: "cr_..."         # Or workspace_token
   ```

**See Also**: [TOKEN SECURITY](CONFIG.md#security-best-practices)

---

### Issue 2: Rate Limit Exceeded (429 Too Many Requests)

**Symptoms:**

- Error: `429 Too Many Requests` or `rate limit exceeded`
- Operations take longer due to retries (expected behavior)
- Requests slow down and eventually succeed

**How Retry Backoff Works:**

The adapter automatically retries 429 responses with exponential backoff:

- Attempt 1 → Wait 1s → Retry
- Attempt 2 → Wait 2s → Retry
- Attempt 3 → Wait 4s → Retry
- Continues up to `max_retries` (default: 5)

Honors `X-RateLimit-Reset` header when present.

**Solutions:**

1. **Do nothing (automatic handling)**:
   - Most 429 errors resolve automatically
   - Allow 30-60 seconds for retries to complete

2. **Check Vantage rate limit settings**:
   - Log into Vantage console
   - Review workspace rate limit configuration
   - Upgrade subscription if limits too restrictive

3. **Reduce request frequency**:

   ```yaml
   params:
     page_size: 10000        # Increase (larger pages = fewer requests)
     request_timeout_seconds: 120  # Allow time for retries
     max_retries: 10         # Increase retry attempts
   ```

4. **Reduce data dimensionality**:

   ```yaml
   params:
     group_bys:
       - provider            # Use fewer dimensions
       - service
       # Remove: account, project, region, resource_id, tags
   ```

5. **Schedule off-peak**:
   - Run backfills during low-usage hours
   - Daily incremental syncs less likely to hit limits

---

### Issue 3: Invalid Configuration

**Symptoms:**

- Error: `config file not found`, `invalid format`, or validation error
- Startup fails before API calls

**Common Configuration Errors:**

| Error | Cause | Solution |
|---|---|---|
| `config file not found` | Path incorrect | `test -f config.yaml` |
| `failed to parse YAML` | Invalid syntax | `yamllint config.yaml` |
| `token is required` | Missing token | Set env var or config |
| `workspace/report token required` | Both missing | Add token to YAML |
| `granularity must be day or month` | Invalid value | Use `"day"` or `"month"` |
| `invalid start_date format` | Not ISO date | Format: `YYYY-MM-DD` |
| `end_date before start_date` | Wrong range | Ensure end > start |
| `page_size exceeds 10000` | Too large | Max is 10,000 |

**Solutions:**

1. **Validate YAML syntax**:

   ```bash
   yamllint config.yaml
   # Or use online validator: https://www.yamllint.com/
   ```

2. **Check required fields**:

   ```yaml
   version: 0.1                              # Required
   source: vantage                           # Required
   credentials:
     token: ${FINFOCUS_VANTAGE_TOKEN}     # Required
   params:
     cost_report_token: "cr_..."            # Required
     granularity: "day"                     # Required
   ```

3. **Verify date format**:

   ```yaml
   params:
     start_date: "2024-01-01"               # YYYY-MM-DD
     end_date: "2024-12-31"                 # YYYY-MM-DD
   ```

4. **Enable debug output**:

   ```bash
   VANTAGE_DEBUG=1 finfocus-vantage pull --config config.yaml
   ```

---

### Issue 4: Pagination Errors (Cursor Issues)

**Symptoms:**

- Error: `cursor expired`, `invalid cursor`, or pagination failure
- Large syncs fail midway through
- Inconsistent record counts

**Causes:**

- API cursor becomes invalid/stale
- Page size too large
- Timeout during pagination
- Concurrent syncs interfering

**Solutions:**

1. **Reduce page size**:

   ```yaml
   params:
     page_size: 1000        # Conservative; default 5000
   ```

2. **Increase timeout**:

   ```yaml
   params:
     request_timeout_seconds: 120  # Allow more time
   ```

3. **Use smaller date ranges**:

   ```bash
   # Instead of full year, import monthly
   for month in {01..12}; do
     finfocus-vantage pull \
       --config config.yaml \
       --start-date "2024-$month-01" \
       --end-date "2024-$month-31"
   done
   ```

4. **Check for concurrent runs**:

   ```bash
   # Ensure only one instance running per report
   ps aux | grep finfocus-vantage
   ```

5. **Enable verbose logging**:
   - See "Enable Verbose Logging" section

---

### Issue 5: Missing or Null Fields

**Symptoms:**

- Unexpected `null` values in cost fields
- Missing usage quantities or cost amounts
- Empty labels or tags

**Causes:**

- Field not available from Vantage
- Metric not requested in config
- Data not yet available (late posting)
- Tag not present in cost report

**Solutions:**

1. **Verify requested metrics**:

   ```yaml
   params:
     metrics:
       - cost                 # Basic
       - usage
       - amortized_cost      # Add if needed
       - taxes               # Add if applicable
   ```

2. **Check Vantage data availability**:
   - Log into Vantage console
   - Verify cost report includes required metrics
   - Check if dimensions available for your provider

3. **Account for late posting**:
   - Cost data takes 2-3 days to finalize
   - Use incremental window D-3 to D-1
   - Final reconciliation may lag

4. **Verify tag configuration**:

   ```yaml
   params:
     group_bys:
       - tags                # Must be present for tags
     tag_prefix_filters:
       - "user:"             # Only include matching prefixes
   ```

---

### Issue 6: Connection Timeout

**Symptoms:**

- Error: `timeout`, `connection refused`, `deadline exceeded`
- Requests hang then fail
- No response from API

**Causes:**

- Network connectivity issues
- Vantage API unreachable or slow
- Configured timeout too short
- Firewall blocking requests

**Solutions:**

1. **Increase timeout**:

   ```yaml
   params:
     request_timeout_seconds: 120  # Increase from 60
   ```

2. **Test network connectivity**:

   ```bash
   # Test DNS
   nslookup api.vantage.sh

   # Test HTTP
   curl -I https://api.vantage.sh/

   # Test with auth
   curl -H "Authorization: Bearer $FINFOCUS_VANTAGE_TOKEN" \
     https://api.vantage.sh/costs
   ```

3. **Check firewall**:
   - Ensure outbound HTTPS (port 443) allowed
   - Verify no proxy interfering
   - Check corporate firewall rules

4. **Check Vantage status**:
   - Visit Vantage status page for incidents
   - Contact Vantage support if API down

---

### Issue 7: Data Duplication or Missing Records

**Symptoms:**

- Duplicate cost records found
- Missing entire days/months of data
- Record counts don't match expected

**Causes:**

- Incremental sync window overlapping
- Multiple concurrent syncs running
- Bookmark not persisting
- Partial failure not recovered

**Solutions:**

1. **Verify sync window**:
   - Default: D-3 to D-1 (accounts for late posting)
   - Ensure no overlap between runs
   - Check cron schedule consistency

2. **Ensure single sync instance**:

   ```bash
   ps aux | grep finfocus-vantage
   # Kill any stale processes
   pkill -f finfocus-vantage
   ```

3. **Verify idempotency**:
   - Same inputs = same idempotency keys
   - Duplicate records same key
   - Sink should deduplicate

4. **Check bookmark storage**:
   - Verify sink persists `last_successful_end_date`
   - Check logs for bookmark updates

---

### Issue 8: Tag/Field Mapping Issues

**Symptoms:**

- Tags missing from output
- Unexpected tag normalization (keys changed)
- Labels malformed or inconsistent

**Causes:**

- Tags not in Vantage report
- Tag filters excluding tags
- Field not in `group_bys`
- Normalization applied incorrectly

**Solutions:**

1. **Verify tags in Vantage**:
   - Check Vantage console for tag availability
   - Confirm `tags` in `group_bys` configuration

2. **Review tag filters**:

   ```yaml
   params:
     tag_prefix_filters:
       - "user:"              # Only user: tags
       - "kubernetes.io/"     # Only k8s tags
       # Empty list = include all tags
   ```

3. **Understand field mapping**:
   - Tag normalization: `CostCenter` → `cost-center`
   - Original values preserved in `labels_raw`
   - Missing tags → empty `labels` map

4. **Check group_bys configuration**:

   ```yaml
   params:
     group_bys:
       - provider
       - service
       - region
       - tags                 # MUST be present
   ```

---

### Issue 9: Memory or Performance Issues

**Symptoms:**

- Adapter consumes excessive memory
- Adapter runs slowly
- Out-of-memory (OOM) error

**Causes:**

- Page size too large
- Too many dimensions/tags
- Large date range without chunking
- Insufficient system resources

**Solutions:**

1. **Reduce page size**:

   ```yaml
   params:
     page_size: 1000        # Conservative: 1000 (vs 5000 default)
   ```

2. **Reduce dimensions**:

   ```yaml
   params:
     group_bys:
       - provider           # Use fewer dimensions
       - service
       # Remove: account, project, region, resource_id, tags
   ```

3. **Chunk large imports**:

   ```bash
   # Import monthly
   for m in {01..12}; do
     finfocus-vantage pull \
       --config config.yaml \
       --start-date "2024-$m-01"
   done
   ```

4. **Monitor resources**:

   ```bash
   watch -n 1 'ps aux | grep finfocus-vantage'
   ```

---

### Issue 10: Wiremock Mock Server Issues

**Symptoms:**

- Mock server won't start
- Tests fail with connection errors
- Mock recordings missing

**Causes:**

- Docker not running
- Port already in use
- Mock mappings incorrect
- Request doesn't match mapping

**Solutions:**

1. **Start mock server**:

   ```bash
   make wiremock-up
   # Or manually:
   docker run -it --rm -p 8080:8080 \
     -v $(pwd)/test/wiremock:/home/wiremock \
     wiremock/wiremock:3
   ```

2. **Verify server running**:

   ```bash
   curl http://localhost:8080/__admin/mappings
   # Should return mock mappings list
   ```

3. **Check port availability**:

   ```bash
   lsof -i :8080
   # Kill if needed: kill -9 <pid>
   ```

4. **Update mock mappings**:
   - Edit `test/wiremock/mappings/`
   - Restart: `make wiremock-down && make wiremock-up`

---

### Issue 11: Unresponsive or Stalled Sync

**Symptoms:**

- Sync process hangs without error
- Progress stops for extended period
- CPU/network activity stops

**Causes:**

- Deadlock waiting for response
- Resource exhaustion
- Network connectivity lost
- Process stuck on pagination

**Solutions:**

1. **Monitor progress**:

   ```bash
   # Watch process in real-time
   watch -n 1 'ps aux | grep finfocus-vantage'
   ```

2. **Graceful shutdown**:

   ```bash
   # First attempt (clean shutdown)
   kill -TERM $(pgrep -f finfocus-vantage)

   # Force kill if needed
   kill -KILL $(pgrep -f finfocus-vantage)
   ```

3. **Resume from checkpoint**:
   - Check bookmark for last successful point
   - Resume from that date/state
   - Use smaller batches for retry

---

### Issue 12: Cost Data Discrepancies

**Symptoms:**

- Adapter cost totals don't match Vantage console
- Missing or unexpected costs
- Amortized vs net cost mismatch

**Causes:**

- Date range differences
- Granularity mismatch
- Filtering differences (group_bys, tags)
- Late posting (data still arriving)
- Metric availability by provider

**Solutions:**

1. **Verify date range**:
   - Confirm start_date and end_date in config
   - Check Vantage console uses same dates

2. **Check granularity**:
   - Daily vs monthly affects buckets
   - Ensure configuration matches expectations

3. **Verify filters**:
   - Same group_bys and metrics
   - Same tag filters applied
   - Same dimensions selected

4. **Account for timing**:
   - Cost data lags 2-3 days
   - Check when Vantage data finalized
   - Use historical data after 3+ days

---

## Enable Verbose Logging

To troubleshoot issues, enable verbose logging to see detailed information about
API calls, retries, and internal operations:

```bash
# Set debug environment variable
export VANTAGE_DEBUG=1

# Run adapter
finfocus-vantage pull --config config.yaml

# Output will include:
# - API request details
# - Response codes and headers
# - Retry attempts and backoff
# - Field mapping details
# - Bookmark operations
```

**Note**: Token values are always redacted from logs for security.

---

## Capture Wiremock Recordings

To record real API interactions for testing:

1. **Start recording mode**:

   ```bash
   docker run -it --rm \
     -p 8080:8080 \
     -e WIREMOCK_OPTS="--record-mappings" \
     -v $(pwd)/test/wiremock:/home/wiremock \
     wiremock/wiremock:3
   ```

2. **Update adapter to use mock**:

   ```yaml
   # Point to local mock instead of api.vantage.sh
   params:
     api_endpoint: "http://localhost:8080"
   ```

3. **Run adapter against mock**:

   ```bash
   export FINFOCUS_VANTAGE_TOKEN="test_token"
   finfocus-vantage pull --config config.yaml
   ```

4. **Mock recordings saved**:
   - Recorded mappings saved to `test/wiremock/mappings/`
   - Review: `cat test/wiremock/mappings/*.json | jq`

5. **Redact sensitive data**:
   - Remove token values from recordings
   - Replace real IDs with generic values
   - Update timestamps to fixed values

---

## Getting Help

If issue not resolved:

1. **Check logs** with `VANTAGE_DEBUG=1`
2. **Review this guide** for similar issues
3. **Check GitHub issues** for known problems
4. **Contact Vantage support** for API-level issues
5. **Open GitHub issue** with:
   - Error message (redacted)
   - Configuration (redacted)
   - Steps to reproduce
   - Debug logs (with tokens redacted)

---

## See Also

- [CONFIG.md](CONFIG.md) - Configuration reference
- [FORECAST.md](FORECAST.md) - Forecast snapshot feature
- [Design Document](../pulumi_cost_vantage_adapter_design_draft_v_0.md) -
  Architecture details
