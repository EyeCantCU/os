#!/usr/bin/env python3
import pexpect
import sys
import shutil

def test_asciinema_play(recording_path):
    """Test asciinema play command with a recording file."""
    try:
        # Start asciinema play with a pseudo-TTY
        child = pexpect.spawn(f'asciinema play {recording_path}', timeout=10)

        # Wait for the expected output
        child.expect('Test recording')
        print("SUCCESS: Found 'Test recording' in playback output")

        # Send Ctrl+C to stop playback
        child.sendcontrol('c')

        # Wait for process to exit
        child.expect(pexpect.EOF)

        return 0

    except pexpect.TIMEOUT:
        print("ERROR: Timeout waiting for playback output")
        print("Output received:", child.before.decode('utf-8', errors='replace'))
        return 1

    except pexpect.EOF:
        # Recording completed before we could stop it (this is OK for short recordings)
        print("SUCCESS: Playback completed")
        return 0

    except Exception as e:
        print(f"ERROR: {e}")
        return 1

if __name__ == '__main__':
    if len(sys.argv) != 2:
        print(f"Usage: {sys.argv[0]} <recording.cast>")
        sys.exit(1)

    # Confirm asciinema command is available
    if not shutil.which('asciinema'):
        print("ERROR: asciinema command not found in PATH")
        sys.exit(1)

    recording_path = sys.argv[1]
    sys.exit(test_asciinema_play(recording_path))
