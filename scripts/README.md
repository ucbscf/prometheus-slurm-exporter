# Mock SLURM Commands for Testing

This directory contains mock implementations of SLURM commands for testing purposes. These are useful for:

- **CI/CD pipelines** where SLURM is not available
- **Local development** without a SLURM cluster
- **Unit testing** with predictable outputs

## Available Mocks

### `mock-sinfo`
Simulates the `sinfo` command with support for:
- JSON output (`--json`)
- CPU summary (`-h -o %C`) 
- Per-node details (`-h -N -O`)
- GPU information in JSON format

### `mock-squeue`
Simulates the `squeue` command with various output formats:
- User job listing
- Account-based grouping
- Job state filtering
- Custom output formats

### `mock-sdiag`
Simulates the `sdiag` command providing:
- Scheduler statistics
- Backfill information
- Performance metrics

## Usage

### In GitHub Actions
The workflow automatically sets up these mocks by creating symlinks in `/usr/local/bin/`.

### Local Testing
```bash
# Add to your PATH
export PATH="$(pwd)/scripts:$PATH"

# Or create symlinks
ln -sf $(pwd)/scripts/mock-sinfo /usr/local/bin/sinfo
ln -sf $(pwd)/scripts/mock-squeue /usr/local/bin/squeue
ln -sf $(pwd)/scripts/mock-sdiag /usr/local/bin/sdiag

# Run tests
go test -v ./...
```

### Manual Testing
```bash
# Test individual commands
./scripts/mock-sinfo --json
./scripts/mock-squeue -a -r -h
./scripts/mock-sdiag
```

## Extending the Mocks

To add support for new command-line options:

1. Edit the appropriate mock script
2. Add a new case in the argument parsing section
3. Provide realistic output that matches real SLURM behavior
4. Update tests if needed

## Real-world Data

The mock outputs are based on real SLURM command outputs but simplified for testing. They include:

- Realistic node names and states
- Typical resource allocations
- Common job states and reasons
- Representative performance metrics

This approach allows other SLURM-related projects to easily adopt similar testing strategies.
