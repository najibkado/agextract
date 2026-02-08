import hashlib
import json
from collections import Counter
from datetime import date, timedelta

from django.contrib.auth import authenticate, login, logout
from django.contrib.auth.decorators import login_required
from django.contrib.auth.models import User
from django.db.models import Count, Sum, Q
from django.shortcuts import render, redirect, get_object_or_404
from django.utils import timezone
from .forms import UploadSessionForm
from .models import Session, Step, SteeringTag
from .parser import TranscriptParser


def upload_view(request):
    if request.method == 'POST':
        form = UploadSessionForm(request.POST, request.FILES)
        if form.is_valid():
            uploaded_file = request.FILES['file']
            content = uploaded_file.read()
            content_hash = hashlib.sha256(content).hexdigest()

            # Dedup: return existing session if same content already uploaded
            if request.user.is_authenticated:
                existing = Session.objects.filter(
                    user=request.user,
                    content_hash=content_hash,
                ).first()
                if existing:
                    return redirect('session_detail', session_id=existing.id)

            parser = TranscriptParser(content)
            session = parser.parse(title=uploaded_file.name)

            # Associate session with logged-in user and store hash
            if request.user.is_authenticated:
                session.user = request.user
            session.content_hash = content_hash
            session.save()

            return redirect('session_detail', session_id=session.id)
    else:
        form = UploadSessionForm()

    return render(request, 'core/upload.html', {'form': form})


def web_login(request):
    if request.user.is_authenticated:
        return redirect('dashboard')

    error = None
    if request.method == 'POST':
        username = request.POST.get('username', '')
        password = request.POST.get('password', '')
        user = authenticate(request, username=username, password=password)
        if user is not None:
            login(request, user)
            next_url = request.POST.get('next') or request.GET.get('next') or 'dashboard'
            return redirect(next_url)
        error = 'Invalid username or password.'

    return render(request, 'core/login.html', {
        'error': error,
        'next': request.GET.get('next', ''),
    })


def web_logout(request):
    logout(request)
    return redirect('upload')


@login_required(login_url='/login/')
def dashboard(request):
    sessions = Session.objects.filter(user=request.user).order_by('-uploaded_at')

    # Dashboard summary stats
    total_sessions = sessions.count()
    total_steps = Step.objects.filter(session__user=request.user).count()
    now = timezone.now()
    sessions_this_month = sessions.filter(
        uploaded_at__year=now.year, uploaded_at__month=now.month
    ).count()

    return render(request, 'core/dashboard.html', {
        'sessions': sessions,
        'total_sessions': total_sessions,
        'total_steps': total_steps,
        'sessions_this_month': sessions_this_month,
    })


def public_profile(request, username):
    profile_user = get_object_or_404(User, username=username)
    sessions = Session.objects.filter(user=profile_user).order_by('-uploaded_at')

    # Compute profile stats
    total_steps = Step.objects.filter(session__user=profile_user).count()
    user_steps = Step.objects.filter(session__user=profile_user, role='user').count()
    agent_steps = Step.objects.filter(session__user=profile_user, role='agent').count()
    system_steps = Step.objects.filter(session__user=profile_user, role='system').count()
    tool_calls = Step.objects.filter(session__user=profile_user, step_type='tool_call').count()
    tags_count = SteeringTag.objects.filter(step__session__user=profile_user).count()

    # Source breakdown
    source_counts = dict(
        sessions.values_list('source').annotate(c=Count('id')).values_list('source', 'c')
    )

    # Steering ratio (how actively the person guides the AI)
    steering_ratio = round(user_steps / max(agent_steps, 1) * 100)

    # --- Chart data ---

    # Activity heatmap: session counts per day for last 365 days
    today = date.today()
    year_ago = today - timedelta(days=364)
    activity_qs = (
        sessions.filter(uploaded_at__date__gte=year_ago)
        .values('uploaded_at__date')
        .annotate(count=Count('id'))
    )
    activity_data = {str(row['uploaded_at__date']): row['count'] for row in activity_qs}

    # Role distribution for donut chart
    role_distribution = {
        'user': user_steps,
        'agent': agent_steps,
        'system': system_steps,
    }

    # Source distribution for horizontal bar
    source_display = {
        'claudecode': 'Claude Code',
        'cursor': 'Cursor',
        'windsurf': 'Windsurf',
        'copilot': 'GitHub Copilot',
        'upload': 'Upload',
    }
    source_distribution = {source_display.get(k, k): v for k, v in source_counts.items()}

    # Sessions over time: last 12 months
    twelve_months_ago = today - timedelta(days=365)
    sessions_by_month = (
        sessions.filter(uploaded_at__date__gte=twelve_months_ago)
        .extra(select={'month': "strftime('%%Y-%%m', uploaded_at)"})
        .values('month')
        .annotate(count=Count('id'))
        .order_by('month')
    )
    # Build full 12-month series
    months_list = []
    for i in range(12):
        m = (today.replace(day=1) - timedelta(days=30 * (11 - i)))
        months_list.append(m.strftime('%Y-%m'))
    month_counts = {row['month']: row['count'] for row in sessions_by_month}
    sessions_over_time = [
        {'month': m, 'count': month_counts.get(m, 0)} for m in months_list
    ]

    # Step type distribution for donut
    step_type_qs = (
        Step.objects.filter(session__user=profile_user)
        .values('step_type')
        .annotate(count=Count('id'))
    )
    step_type_distribution = {row['step_type']: row['count'] for row in step_type_qs}

    return render(request, 'core/public_profile.html', {
        'profile_user': profile_user,
        'sessions': sessions,
        'total_steps': total_steps,
        'user_steps': user_steps,
        'agent_steps': agent_steps,
        'tool_calls': tool_calls,
        'tags_count': tags_count,
        'source_counts': source_counts,
        'steering_ratio': steering_ratio,
        'activity_data_json': json.dumps(activity_data),
        'role_distribution_json': json.dumps(role_distribution),
        'source_distribution_json': json.dumps(source_distribution),
        'sessions_over_time_json': json.dumps(sessions_over_time),
        'step_type_distribution_json': json.dumps(step_type_distribution),
    })


def session_detail(request, session_id):
    session = get_object_or_404(Session, id=session_id)
    steps = session.steps.all().prefetch_related('tags')

    # Session-level stats
    total = steps.count()
    user_count = steps.filter(role='user').count()
    agent_count = steps.filter(role='agent').count()
    tool_count = steps.filter(step_type='tool_call').count()
    tag_count = SteeringTag.objects.filter(step__session=session).count()
    steering_ratio = round(user_count / max(agent_count, 1) * 100)

    # Conversation flow: group steps in chunks of 20
    chunk_size = 20
    steps_list = list(steps.values('role', 'step_type', 'order'))
    conversation_flow = []
    for i in range(0, max(len(steps_list), 1), chunk_size):
        chunk = steps_list[i:i + chunk_size]
        chunk_num = i // chunk_size + 1
        label = f"{i + 1}-{min(i + chunk_size, len(steps_list))}"
        conversation_flow.append({
            'chunk': label,
            'user': sum(1 for s in chunk if s['role'] == 'user'),
            'agent': sum(1 for s in chunk if s['role'] == 'agent' and s['step_type'] != 'tool_call'),
            'tool': sum(1 for s in chunk if s['step_type'] == 'tool_call'),
            'system': sum(1 for s in chunk if s['role'] == 'system'),
        })

    return render(request, 'core/session_detail.html', {
        'session': session,
        'steps': steps,
        'total_steps': total,
        'user_count': user_count,
        'agent_count': agent_count,
        'tool_count': tool_count,
        'tag_count': tag_count,
        'steering_ratio': steering_ratio,
        'conversation_flow_json': json.dumps(conversation_flow),
    })

def add_tag(request, step_id):
    # HTMX view to add a tag
    # Simplified for MVP: Just adds a "Pivot" tag for now or toggles
    step = get_object_or_404(Step, id=step_id)
    if request.method == 'POST':
        tag_type = request.POST.get('tag_type', 'pivot')
        SteeringTag.objects.create(step=step, tag_type=tag_type)
        return render(request, 'core/partials/step_tags.html', {'step': step})
    return HttpResponse(status=405)

def step_card(request, step_id):
    step = get_object_or_404(Step, id=step_id)
    return render(request, 'core/partials/step_card.html', {'step': step})
