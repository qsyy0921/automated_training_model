"""Python Agent Runtime prototype.

This package owns the future LLM-heavy agent loop: intent planning, skill
selection, tool-call planning, and multimodal reasoning. Go remains the control
plane and calls this runtime through a stable JSON envelope.
"""

