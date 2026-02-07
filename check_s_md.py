import os
import django

os.environ.setdefault('DJANGO_SETTINGS_MODULE', 'agextract.settings')
django.setup()

from core.parser import TranscriptParser
from core.models import Session

def test_s_md():
    with open('s.md', 'r') as f:
        content = f.read()
    
    parser = TranscriptParser(content)
    session = parser.parse(title="Real Transcript Test")
    
    print(f"Session: {session.title}")
    print(f"Steps: {session.steps.count()}")
    
    for step in session.steps.all()[:10]:
        print(f"[{step.order}] {step.role} ({step.step_type}): {step.content[:40]}...")

if __name__ == '__main__':
    test_s_md()
