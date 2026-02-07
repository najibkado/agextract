from collections import Counter

from django.contrib.auth import authenticate, login, logout
from django.contrib.auth.decorators import login_required
from django.contrib.auth.models import User
from django.db.models import Count, Sum, Q
from django.shortcuts import render, redirect, get_object_or_404
from .forms import UploadSessionForm
from .models import Session, Step, SteeringTag
from .parser import TranscriptParser


def upload_view(request):
    if request.method == 'POST':
        form = UploadSessionForm(request.POST, request.FILES)
        if form.is_valid():
            uploaded_file = request.FILES['file']
            content = uploaded_file.read()

            parser = TranscriptParser(content)
            session = parser.parse(title=uploaded_file.name)

            # Associate session with logged-in user
            if request.user.is_authenticated:
                session.user = request.user
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
    return render(request, 'core/dashboard.html', {'sessions': sessions})


def public_profile(request, username):
    profile_user = get_object_or_404(User, username=username)
    sessions = Session.objects.filter(user=profile_user).order_by('-uploaded_at')
    return render(request, 'core/public_profile.html', {
        'profile_user': profile_user,
        'sessions': sessions,
    })

def session_detail(request, session_id):
    session = get_object_or_404(Session, id=session_id)
    steps = session.steps.all().prefetch_related('tags')
    return render(request, 'core/session_detail.html', {
        'session': session,
        'steps': steps
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
