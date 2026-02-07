from django.conf import settings
from django.db import models
import uuid

class Session(models.Model):
    """
    Represents a parsed AI coding session (e.g., from a Cursor export).
    """
    SOURCE_CHOICES = [
        ('claudecode', 'Claude Code'),
        ('cursor', 'Cursor'),
        ('windsurf', 'Windsurf'),
        ('copilot', 'GitHub Copilot'),
        ('upload', 'Manual Upload'),
    ]

    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    title = models.CharField(max_length=255, help_text="e.g. 'Build a React Todo App'")
    uploaded_at = models.DateTimeField(auto_now_add=True)
    user = models.ForeignKey(
        settings.AUTH_USER_MODEL, on_delete=models.CASCADE,
        null=True, blank=True, related_name='sessions',
    )
    source = models.CharField(max_length=20, choices=SOURCE_CHOICES, default='upload')
    source_session_id = models.CharField(
        max_length=255, blank=True, default='',
        help_text="Original session ID from the source tool",
    )

    # Metadata extracted from the transcript
    duration_seconds = models.IntegerField(null=True, blank=True)
    token_usage = models.IntegerField(null=True, blank=True)
    file_count = models.IntegerField(null=True, blank=True)
    
    # v2 Storyboard fields
    summary = models.TextField(blank=True, help_text="AI-generated or user-written summary")
    hero_moment = models.ForeignKey('Step', on_delete=models.SET_NULL, null=True, blank=True, related_name='hero_sessions')
    
    def __str__(self):
        return self.title

class Step(models.Model):
    """
    A single interaction or event in the session timeline.
    """
    ROLE_CHOICES = [
        ('user', 'User'),
        ('agent', 'Agent'),
        ('system', 'System'),
    ]
    
    STEP_TYPE_CHOICES = [
        ('prompt', 'Prompt'),         # User input
        ('tool_call', 'Tool Call'),   # e.g., view_file, edit_file
        ('diff', 'Code Diff'),        # The actual code change
        ('thought', 'Thinking'),      # Model internal monologue
        ('text', 'Text Response'),    # Normal chat response
    ]
    
    session = models.ForeignKey(Session, on_delete=models.CASCADE, related_name='steps')
    role = models.CharField(max_length=20, choices=ROLE_CHOICES)
    step_type = models.CharField(max_length=20, choices=STEP_TYPE_CHOICES)
    content = models.TextField(help_text="The raw content of the step")
    
    # Ordering and timing
    timestamp = models.DateTimeField(null=True, blank=True)
    order = models.IntegerField(default=0, help_text="Sequence number in the session")
    
    class Meta:
        ordering = ['order']

    def __str__(self):
        return f"{self.role} - {self.step_type} ({self.order})"

class SteeringTag(models.Model):
    """
    User annotations to highlight 'human in the loop' moments.
    """
    TAG_TYPES = [
        ('pivot', 'Pivot'),              # Redirecting the agent
        ('correction', 'Correction'),    # Fixing a hallucination/mistake
        ('architecture', 'Architectural Decision'), # Key design choice
    ]
    
    step = models.ForeignKey(Step, on_delete=models.CASCADE, related_name='tags')
    tag_type = models.CharField(max_length=50, choices=TAG_TYPES)
    comment = models.TextField(blank=True, help_text="Why was this important?")
    created_at = models.DateTimeField(auto_now_add=True)

    # v2 Storyboard fields
    impact_score = models.IntegerField(default=5, help_text="1-10 scale of importance")
    before_snapshot = models.TextField(blank=True, help_text="Code context before the change")
    after_snapshot = models.TextField(blank=True, help_text="Code context after the change")

    def __str__(self):
        return f"{self.tag_type} on Step {self.step.order}"
