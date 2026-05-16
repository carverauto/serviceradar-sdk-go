# SDK Fixtures

These JSON files are stable test fixtures for SDK encode/decode coverage.
They are not runtime defaults and operators are not expected to edit them.

For northbound action fixtures, values such as `reason: "RADIUS auth failure"`
are sample launch inputs used by tests to prove the SDK preserves both private
`input_values` and public `redacted_input_values` shapes. Real values are
created by ServiceRadar when a user, schedule, or event handler launches an
action.
