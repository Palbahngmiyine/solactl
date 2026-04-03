# Instructions

## Read First
- Read the code before writing tests.
- Treat code as the source of truth.
- Do not assume behavior.

## Required
- Test both success and failure paths.
- Test all branches of conditions.
- Test boundary values (nil, empty, zero, min, max).
- Add regression tests for every bug fix.
- Validate invariants, not just errors.
- Ensure tests are deterministic.
- Ensure all resources are cleaned up.
- Use multiple test layers when needed (unit, boundary, fault, concurrency, fuzz).

## Must Verify
- state consistency
- side effects
- idempotency
- rollback correctness
- resource cleanup

## Failure Injection
- Simulate dependency failures.
- Cover first-call, Nth-call, and continuous failures.
- Include timeout and cancellation cases.
- Include partial success followed by failure.

## Concurrency
- Verify no race conditions.
- Verify no deadlocks.
- Verify no duplicate execution.
- Verify ordering and invariants.

## Persistence
- Ensure atomic behavior (all or nothing).
- Ensure no corrupt intermediate state.
- Ensure safe retry and recovery.

## Fuzz
- Apply fuzz tests to input parsing and decoding.
- Ensure no panic or unbounded resource usage.

## Determinism
- Do not rely on sleep-based timing.
- Control time and randomness.
- Use bounded retries.

## Style
- Use table-driven tests where appropriate.
- Use fakes for external dependencies.
- Use cleanup hooks.

## Naming
- Name tests by behavior, edge case, or failure mode.

## Forbidden
- Do not test only happy paths.
- Do not skip edge cases.
- Do not write non-deterministic tests.
- Do not leave resources unverified.
- Do not merge multiple concerns into one test.
- Do not rely only on line coverage.
- Do not ignore failure scenarios.
