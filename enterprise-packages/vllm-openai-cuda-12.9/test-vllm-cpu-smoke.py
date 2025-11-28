import os
from vllm import LLM

# Intentionally minimal: construct LLM so it errors on CPU-only with CUDA wheel
LLM(model="sshleifer/tiny-gpt2")
