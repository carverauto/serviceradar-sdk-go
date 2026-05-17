# SDK Fixtures

These JSON files are stable test fixtures for SDK encode/decode coverage.
They are not runtime defaults and operators are not expected to edit them.

For northbound action fixtures, values such as `reason: "RADIUS auth failure"`
are sample launch inputs used by tests to prove the SDK preserves both private
`input_values` and public `redacted_input_values` shapes. The deferred and poll
fixtures show the asynchronous contract: a provider returns an external task ID
and continuation state, ServiceRadar sends that continuation state back in a
poll request, and the provider eventually returns final per-target results. Real
values are created by ServiceRadar when a user, schedule, or event handler
launches an action.
