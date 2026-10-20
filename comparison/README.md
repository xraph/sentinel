# Comparing captured evaluations

Use `CompareRecordedSets` when you store each trial in its own evaluation job:

```go
result, err := comparison.CompareRecordedSets(ctx, comparison.RecordedSetComparisonRequest{
    Store:        store,
    BaselineIDs:  baselineJobIDs,
    CandidateIDs: candidateJobIDs,
    Seed:         42,
    MinCases:     20,
})
```

Both arms must use the same frozen plan and contain the same case and repetition
identities. You can batch either arm differently or pass job IDs in any order.
The comparison reads the persisted jobs, checks their authenticated scope and
request digests, and rejects repeated jobs or overlapping case/repetition pairs
within an arm. A job cannot appear in both arms.

The result keeps each job ID and request digest in `BaselineJobs` and
`CandidateJobs`. The older single-job fields are populated only when that arm
contains one job. `CompareRecorded` remains available for two batched jobs and
uses the same validation and statistical calculation.

Repetitions are averaged within each case. The bootstrap resamples distinct
cases, so splitting a batch into more jobs cannot increase the sample size.
The seed reproduces the 2,000 bootstrap samples and the 95 percent interval.
Missing pairs are rejected. Pending or failed jobs produce an inconclusive
report without an estimate, and a high mean score does not clear failed required
criteria. Unknown costs stay unavailable.

You still need to check the selected job IDs against your experiment's frozen
roster. Equal subsets on both arms do not prove that every planned case ran.
The comparison does not execute targets or approve production activation.
