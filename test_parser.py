import os
import django

os.environ.setdefault('DJANGO_SETTINGS_MODULE', 'agextract.settings')
django.setup()

from core.parser import TranscriptParser
from core.models import Session

def test_parser():
    with open('sample_transcript.md', 'r') as f:
        content = f.read()
    
    parser = TranscriptParser(content)
    session = parser.parse(title="Test Session")
    
    print(f"Session Created: {session.title}")
    print(f"Total Steps: {session.steps.count()}")
    
    for step in session.steps.all():
        print(f"[{step.order}] {step.role} ({step.step_type}): {step.content[:30]}...")

if __name__ == '__main__':
    test_parser()
