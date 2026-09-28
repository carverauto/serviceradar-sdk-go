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

`plugin_run_overrides_config.json` and
`northbound_action_result_run_overrides.json` cover run overrides: the config
document a scheduled run receives (one active and one expired override) and an
action result that sets one override and ends another. They are shared
byte-for-byte with the Rust SDK so both SDKs decode the same wire shapes.

`grpc_unary_request.json`, `grpc_unary_response_ok.json` and
`grpc_unary_response_error.json` are the `grpc_unary` host ABI request and the
two response shapes (OK and a non-OK status). All hosts, methods and messages
are invented. `credential_grant_oauth2_client_credentials.json` is a credential
broker grant with an `oauth2_client_credentials` inject spec and allow scope.
These are also shared byte-for-byte with the Rust SDK.
