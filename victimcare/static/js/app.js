/* ==========================================================================
   Victim Care — client application
   The server renders the first paint, this file adds the interactive flows:
   authentication, the SOS countdown, reporting, settings, picture uploads,
   theme switching and the moving picture area.
   ========================================================================== */

(function () {
  'use strict';

  const $ = (id) => document.getElementById(id);
  const bootstrap = readBootstrap();

  const state = {
    signedIn: !!bootstrap.signedIn,
    user: bootstrap.user || {},
    csrf: bootstrap.csrf || '',
    pictures: bootstrap.pictures || [],
    contact: bootstrap.contact || {},
    contacts: bootstrap.contacts || [],
    reports: bootstrap.reports || [],
    tab: 'home',
    authMode: 'login',
    reportType: 'Accident',
    countdownTimer: null,
    countdownSeconds: 5,
    practice: false,
    lastMessage: ''
  };

  const views = ['home', 'report', 'aid', 'contacts', 'settings'];
  let countryCache = null;

  /* ------------------------------------------------------------ helpers */

  function readBootstrap() {
    const node = document.getElementById('bootstrap');
    if (!node) return {};
    try {
      return JSON.parse(node.textContent || '{}');
    } catch (e) {
      return {};
    }
  }

  async function api(path, options) {
    const opts = Object.assign({ method: 'GET', headers: {} }, options || {});
    opts.headers = Object.assign({}, opts.headers);
    if (state.csrf && opts.method !== 'GET') opts.headers['X-CSRF-Token'] = state.csrf;

    const response = await fetch(path, opts);
    let payload = {};
    try {
      payload = await response.json();
    } catch (e) {
      payload = {};
    }
    if (!response.ok) {
      const error = new Error(payload.error || 'Something went wrong. Please try again.');
      error.field = payload.field || '';
      error.status = response.status;
      throw error;
    }
    return payload;
  }

  function postJSON(path, body) {
    return api(path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    });
  }

  function putJSON(path, body) {
    return api(path, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    });
  }

  function patchJSON(path, body) {
    return api(path, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    });
  }

  function setStatus(element, message, isError) {
    if (!element) return;
    element.textContent = message || '';
    element.classList.toggle('error', !!isError);
  }

  /* -------------------------------------------------- country and state */

  // loadCountries fetches the country and state list once and keeps it.
  async function loadCountries() {
    if (countryCache) return countryCache;
    try {
      const result = await api('/api/v1/countries');
      countryCache = result.countries || [];
    } catch (e) {
      countryCache = [];
    }
    return countryCache;
  }

  // fillStates rebuilds the state dropdown for the country that is selected.
  // Countries without a known list get a single "type it" option.
  async function fillStates(countrySelect, stateSelect, selected) {
    if (!countrySelect || !stateSelect) return;
    const country = countrySelect.value;
    const list = await loadCountries();
    const match = list.find((item) => item.name === country);
    const regions = match && match.regions ? match.regions : [];

    stateSelect.innerHTML = '';
    stateSelect.disabled = false;

    const placeholder = document.createElement('option');
    placeholder.value = '';
    placeholder.textContent = regions.length ? 'Choose a state or region' : 'Choose a state or region';
    stateSelect.appendChild(placeholder);

    regions.forEach((region) => {
      const option = document.createElement('option');
      option.value = region;
      option.textContent = region;
      stateSelect.appendChild(option);
    });

    const other = document.createElement('option');
    other.value = '__other__';
    other.textContent = 'Other (not listed)';
    stateSelect.appendChild(other);

    if (selected) {
      const found = Array.prototype.some.call(stateSelect.options, (option) => option.value === selected);
      stateSelect.value = found ? selected : '__other__';
      if (stateSelect.value === '__other__') {
        const custom = document.createElement('option');
        custom.value = selected;
        custom.textContent = selected;
        stateSelect.appendChild(custom);
        stateSelect.value = selected;
      }
    }
  }

  // chosenState returns the state the user picked, resolving the "other" option.
  function chosenState(stateSelect) {
    if (!stateSelect) return '';
    if (stateSelect.value === '__other__') {
      const custom = window.prompt('Type the name of your state or region');
      return custom ? custom.trim() : '';
    }
    return stateSelect.value.trim();
  }

  /* ---------------------------------------------------- password rules */

  function passwordChecks(password) {
    return {
      length: password.length >= 8,
      letter: /[A-Za-z]/.test(password),
      digit: /[0-9]/.test(password)
    };
  }

  function updatePasswordRules() {
    const input = $('signup-password');
    if (!input) return;
    const password = input.value;
    const checks = passwordChecks(password);
    document.querySelectorAll('#password-rules li').forEach((item) => {
      const rule = item.dataset.rule;
      item.classList.toggle('is-met', !!checks[rule]);
    });

    const meter = $('strength-meter');
    if (!meter) return;
    meter.hidden = password.length === 0;
    const score = Object.keys(checks).filter((key) => checks[key]).length;
    const fill = $('strength-fill');
    const label = $('strength-label');
    if (!fill || !label) return;
    const percent = (score / 3) * 100;
    fill.style.width = percent + '%';
    fill.dataset.score = String(score);
    label.textContent = score === 3 ? 'Strong' : score === 2 ? 'Fair' : 'Weak';
  }

  /* -------------------------------------------------------- app screens */

  function showScreen(screen) {
    $('welcome').hidden = screen !== 'welcome';
    $('auth-panel').hidden = screen !== 'auth';
    $('signed-in-view').hidden = screen !== 'app';
    $('top-nav').hidden = screen !== 'app';
    $('signed-in-actions').hidden = screen !== 'app';
  }

  function switchTab(next) {
    state.tab = next;
    views.forEach((name) => { $(name + '-view').hidden = name !== next; });
    document.querySelectorAll('[data-tab]').forEach((button) => {
      if (button.dataset.tab === next) button.setAttribute('aria-current', 'page');
      else button.removeAttribute('aria-current');
    });
    if (window.history && window.history.replaceState) {
      window.history.replaceState(null, '', '#' + next);
    }
    if (next === 'report') renderReports();
    if (next === 'contacts') {
      renderContacts();
      resetContactForm();
      $('emergency-call').href = 'tel:' + (state.user.emergencyNumber || '112');
    }
    if (next === 'settings') fillSettings();
    if (next === 'home') updateSosLabel();
  }

  function updateSosLabel() {
    const seconds = state.user.countdownSeconds || 5;
    const text = $('.sos small');
    if (text) text.textContent = 'Press for help · ' + seconds + 's to cancel';
  }

  /* ---------------------------------------------------------- auth flow */

  function openAuth(mode) {
    state.authMode = mode;
    $('prototype-form').reset();
    $('auth-error').textContent = '';
    $('login-head').hidden = mode !== 'login';
    $('signup-head').hidden = mode !== 'signup';
    $('login-fields').hidden = mode !== 'login';
    $('signup-fields').hidden = mode !== 'signup';
    $('complete-login').hidden = mode !== 'login';
    $('complete-signup').hidden = mode !== 'signup';
    showScreen('auth');
    const first = mode === 'login' ? $('prototype-email') : $('signup-first-name');
    if (first) first.focus();
  }

  async function submitAuth(event) {
    event.preventDefault();
    const errorBox = $('auth-error');
    errorBox.textContent = '';

    try {
      if (state.authMode === 'signup') {
        const payload = {
          firstName: $('signup-first-name').value.trim(),
          secondName: $('signup-second-name').value.trim(),
          email: $('signup-email').value.trim(),
          country: $('signup-country').value,
          state: chosenState($('signup-state')),
          nation: $('signup-nation').value.trim(),
          password: $('signup-password').value
        };
        if (!payload.firstName || !payload.secondName || !payload.email || !payload.country ||
            !payload.state || !payload.nation || !payload.password) {
          errorBox.textContent = 'Complete every field to continue.';
          return;
        }
        const checks = passwordChecks(payload.password);
        if (!checks.length || !checks.letter || !checks.digit) {
          errorBox.textContent = 'Password is incorrect. It needs at least 8 characters, with letters and numbers.';
          return;
        }
        if ($('signup-password').value !== $('signup-confirm-password').value) {
          errorBox.textContent = 'Passwords do not match.';
          return;
        }
        const result = await postJSON('/api/v1/auth/signup', payload);
        state.user = result.user;
      } else {
        const email = $('prototype-email').value.trim();
        const password = $('prototype-password').value;
        if (!email || !password) {
          errorBox.textContent = 'Enter your email address and password to continue.';
          return;
        }
        const result = await postJSON('/api/v1/auth/login', { email: email, password: password });
        state.user = result.user;
      }
      await afterSignIn();
    } catch (error) {
      errorBox.textContent = error.message;
    }
  }

  async function afterSignIn() {
    const me = await api('/api/v1/auth/me');
    state.user = me.user;
    state.csrf = me.csrf;
    state.pictures = me.pictures || [];
    state.signedIn = true;
    applyTheme(state.user.theme);
    showScreen('app');
    switchTab('home');
    $('create').focus();
  }

  async function signOut() {
    try {
      await postJSON('/api/v1/auth/logout', {});
    } catch (e) { /* the cookie is cleared by the server anyway */ }
    try {
      localStorage.removeItem('victimcare.theme');
      localStorage.removeItem('victimcare.motion');
    } catch (e) { /* storage is optional */ }
    window.location.href = '/';
  }

  /* ------------------------------------------------------------- alerts */

  function showOverlay(part) {
    $('overlay').hidden = false;
    ['countdown', 'location', 'ready'].forEach((name) => { $(name + '-state').hidden = name !== part; });
  }

  function closeOverlay() {
    clearInterval(state.countdownTimer);
    state.countdownTimer = null;
    state.practice = false;
    $('overlay').hidden = true;
    switchTab('home');
    $('sos').focus();
  }

  function startCountdown(practice) {
    clearInterval(state.countdownTimer);
    state.practice = !!practice;
    let seconds = state.user.countdownSeconds || 5;
    $('countdown-number').textContent = seconds;
    showOverlay('countdown');
    $('cancel-alert').focus();
    state.countdownTimer = setInterval(() => {
      seconds -= 1;
      if (seconds <= 0) {
        clearInterval(state.countdownTimer);
        state.countdownTimer = null;
        prepareAlert(state.practice ? 'Practice run' : 'SOS emergency', '');
      } else {
        $('countdown-number').textContent = seconds;
      }
    }, 1000);
  }

  function buildMessage(kind, note, position) {
    const name = state.user.fullName || 'Someone';
    let location = 'location not shared';
    if (position) {
      location = 'https://maps.google.com/?q=' + position.coords.latitude + ',' + position.coords.longitude;
    }
    return name + ' needs help. ' + kind + '. ' + (note ? note + ' ' : '') + 'Location: ' + location;
  }

  async function prepareAlert(kind, note) {
    const practice = state.practice;
    showOverlay('location');

    const position = await resolveLocation();
    const message = buildMessage(kind, note, position);
    state.lastMessage = message;

    if (!practice) {
      try {
        const result = await postJSON('/api/v1/incidents', {
          kind: kind,
          note: note || '',
          latitude: position ? position.coords.latitude : null,
          longitude: position ? position.coords.longitude : null,
          locationShared: !!position
        });
        state.reports = result.incidents || [];
      } catch (e) {
        /* the alert still has to be usable when the server is unreachable */
      }

      if (state.user.vibrate && navigator.vibrate) navigator.vibrate([200, 100, 200]);
    }

    const number = state.user.emergencyNumber || '112';
    $('call-link').href = 'tel:' + number;
    $('call-label').textContent = 'Call ' + number;

    const primary = primaryContact();
    const hasContact = !!(primary && primary.phone);
    $('text-link').hidden = !hasContact;
    $('no-contact').hidden = hasContact;
    if (hasContact) {
      $('text-link').href = 'sms:' + primary.phone + '?body=' + encodeURIComponent(message);
      $('text-label').textContent = 'Text ' + (primary.name || 'my contact');
    }

    $('ready-title').textContent = practice ? 'Practice run finished' : 'Alert ready';
    $('ready-description').textContent = practice
      ? 'This was a practice run. Nothing was sent and nothing was saved.'
      : 'Choose an option below to finish from your device. Nothing has been sent yet.';

    showOverlay('ready');
    $('close-alert').focus();
  }

  // primaryContact returns the contact that text alerts go to.
  function primaryContact() {
    const list = state.contacts || [];
    if (!list.length) return state.contact && state.contact.phone ? state.contact : null;
    return list.find((contact) => contact.primary) || list[0];
  }

  async function copyAlertMessage() {
    const button = $('copy-alert');
    if (!state.lastMessage) {
      setStatus($('alert-note'), 'There is no message to copy yet.', true);
      return;
    }
    try {
      if (navigator.clipboard && navigator.clipboard.writeText) {
        await navigator.clipboard.writeText(state.lastMessage);
      } else {
        const area = document.createElement('textarea');
        area.value = state.lastMessage;
        document.body.appendChild(area);
        area.select();
        document.execCommand('copy');
        document.body.removeChild(area);
      }
      button.textContent = 'Copied';
      setTimeout(() => { button.textContent = 'Copy the message'; }, 2000);
    } catch (e) {
      button.textContent = 'Copy failed';
      setTimeout(() => { button.textContent = 'Copy the message'; }, 2000);
    }
  }

  function resolveLocation() {
    return new Promise((resolve) => {
      if (!state.user.shareLocation || !navigator.geolocation) {
        resolve(null);
        return;
      }
      navigator.geolocation.getCurrentPosition(
        (position) => resolve(position),
        () => resolve(null),
        { timeout: 8000, enableHighAccuracy: true, maximumAge: 30000 }
      );
    });
  }

  /* ------------------------------------------------------------ reports */

  function renderReports() {
    const list = $('reports-list');
    if (!list) return;
    list.innerHTML = '';
    $('reports-empty').hidden = state.reports.length > 0;
    state.reports.forEach((report) => {
      const card = document.createElement('article');
      card.className = 'report-card';

      const type = document.createElement('strong');
      type.textContent = report.kind;

      const note = document.createElement('span');
      note.textContent = report.note || 'No details';

      const time = document.createElement('small');
      time.textContent = new Date(report.created_at || report.createdAt || Date.now()).toLocaleString();

      card.append(type, document.createElement('br'), note, document.createElement('br'), time);
      list.appendChild(card);
    });
  }

  /* ----------------------------------------------------------- contacts */

  function renderContacts() {
    const list = $('contact-list');
    if (!list) return;
    list.innerHTML = '';
    state.contacts.forEach((contact) => {
      const item = document.createElement('article');
      item.className = 'contact-item' + (contact.primary ? ' is-primary' : '');
      item.dataset.id = contact.id;

      const info = document.createElement('div');
      const name = document.createElement('strong');
      name.className = 'contact-name';
      name.textContent = contact.name;
      const phone = document.createElement('span');
      phone.className = 'muted contact-phone';
      phone.textContent = contact.phone;
      info.append(name, phone);

      const actions = document.createElement('div');
      actions.className = 'contact-actions';

      if (contact.primary) {
        const badge = document.createElement('span');
        badge.className = 'badge badge-primary';
        badge.textContent = 'Primary';
        actions.appendChild(badge);
      } else {
        const primary = document.createElement('button');
        primary.type = 'button';
        primary.className = 'link-button';
        primary.dataset.contactAction = 'primary';
        primary.dataset.id = contact.id;
        primary.textContent = 'Make primary';
        actions.appendChild(primary);
      }

      const edit = document.createElement('button');
      edit.type = 'button';
      edit.className = 'link-button';
      edit.dataset.contactAction = 'edit';
      edit.dataset.id = contact.id;
      edit.dataset.name = contact.name;
      edit.dataset.phone = contact.phone;
      edit.textContent = 'Edit';

      const remove = document.createElement('button');
      remove.type = 'button';
      remove.className = 'link-button danger';
      remove.dataset.contactAction = 'delete';
      remove.dataset.id = contact.id;
      remove.textContent = 'Delete';

      actions.append(edit, remove);
      item.append(info, actions);
      list.appendChild(item);
    });
    $('contacts-empty').hidden = state.contacts.length > 0;
  }

  function resetContactForm() {
    $('contact-id').value = '';
    $('contact-name').value = '';
    $('contact-phone').value = '';
    $('contact-form-title').textContent = 'Add a contact';
    $('cancel-contact-edit').hidden = true;
  }

  async function contactAction(action, id, name, phone) {
    const status = $('contact-status');
    try {
      let result;
      if (action === 'primary') {
        result = await postJSON('/api/v1/contacts/' + id + '/primary', {});
      } else if (action === 'delete') {
        result = await api('/api/v1/contacts/' + id, { method: 'DELETE' });
      } else {
        result = await postJSON('/api/v1/contacts', { id: id, name: name, phone: phone });
      }
      state.contacts = result.contacts || [];
      state.contact = primaryContact() || {};
      renderContacts();
      setStatus(status, action === 'primary' ? 'Primary contact updated.' :
        action === 'delete' ? 'Contact removed.' : 'Contact saved.', false);
    } catch (error) {
      setStatus(status, error.message, true);
    }
  }

  /* ----------------------------------------------------------- settings */

  function fillSettings() {
    $('profile-first-name').value = state.user.firstName || '';
    $('profile-second-name').value = state.user.secondName || '';
    $('profile-email').value = state.user.email || '';
    $('profile-nation').value = state.user.nation || '';
    $('profile-country').value = state.user.country || '';
    fillStates($('profile-country'), $('profile-state'), state.user.state || '');
    $('setting-number').value = state.user.emergencyNumber || '112';
    $('setting-location').checked = !!state.user.shareLocation;
    $('setting-vibration').checked = !!state.user.vibrate;
    $('setting-motion').checked = state.user.motion !== false;
    $('setting-countdown').value = String(state.user.countdownSeconds || 5);
    updateThemeSwatches(state.user.theme || 'light');
    updateAvatarPreview();
  }

  function updateAvatarPreview() {
    const preview = $('avatar-preview');
    if (!preview) return;
    const url = state.user.avatarUrl || '';
    preview.innerHTML = '';
    if (url) {
      const img = document.createElement('img');
      img.id = 'avatar-image';
      img.src = url;
      img.alt = 'Your profile picture';
      preview.appendChild(img);
    } else {
      const span = document.createElement('span');
      span.className = 'avatar-initials large';
      span.id = 'avatar-initials';
      span.textContent = state.user.initials || 'VC';
      preview.appendChild(span);
    }
    const button = $('avatar-button');
    if (button) {
      button.innerHTML = '';
      if (url) {
        const img = document.createElement('img');
        img.className = 'avatar-image';
        img.src = url;
        img.alt = '';
        button.appendChild(img);
      } else {
        const span = document.createElement('span');
        span.className = 'avatar-initials';
        span.textContent = state.user.initials || 'VC';
        button.appendChild(span);
      }
    }
  }

  function updateThemeSwatches(theme) {
    document.querySelectorAll('.theme-swatch').forEach((swatch) => {
      swatch.setAttribute('aria-checked', String(swatch.dataset.theme === theme));
    });
  }

  function applyTheme(theme) {
    document.documentElement.setAttribute('data-theme', theme);
    updateThemeSwatches(theme);
    try {
      localStorage.setItem('victimcare.theme', theme);
    } catch (e) { /* storage is optional */ }
  }

  function applyMotion(motion) {
    document.documentElement.setAttribute('data-motion', motion ? 'on' : 'off');
    try {
      localStorage.setItem('victimcare.motion', motion ? 'on' : 'off');
    } catch (e) { /* storage is optional */ }
  }

  async function saveProfile(event) {
    event.preventDefault();
    const status = $('profile-status');
    try {
      const result = await patchJSON('/api/v1/users/me', {
        firstName: $('profile-first-name').value.trim(),
        secondName: $('profile-second-name').value.trim(),
        email: $('profile-email').value.trim(),
        country: $('profile-country').value,
        state: chosenState($('profile-state')),
        nation: $('profile-nation').value.trim(),
        password: $('profile-password').value
      });
      state.user = result.user;
      $('profile-password').value = '';
      updateAvatarPreview();
      setStatus(status, 'Profile updated.', false);
    } catch (error) {
      setStatus(status, error.message, true);
    }
  }

  async function saveSettings(event) {
    event.preventDefault();
    const status = $('settings-status');
    const motion = $('setting-motion').checked;
    try {
      const result = await putJSON('/api/v1/users/me/settings', {
        theme: state.user.theme || 'light',
        motion: motion,
        shareLocation: $('setting-location').checked,
        vibrate: $('setting-vibration').checked,
        emergencyNumber: $('setting-number').value.trim(),
        countdownSeconds: parseInt($('setting-countdown').value, 10)
      });
      state.user = result.settings;
      applyMotion(motion);
      updateSosLabel();
      setStatus(status, 'Alert settings updated.', false);
    } catch (error) {
      setStatus(status, error.message, true);
    }
  }

  async function changeTheme(theme) {
    applyTheme(theme);
    state.user.theme = theme;
    updateThemeSwatches(theme);
    setStatus($('theme-status'), 'Theme changed to ' + theme + '.', false);
    try {
      const result = await putJSON('/api/v1/users/me/settings', {
        theme: theme,
        motion: $('setting-motion').checked,
        shareLocation: $('setting-location').checked,
        vibrate: $('setting-vibration').checked,
        emergencyNumber: $('setting-number').value.trim() || state.user.emergencyNumber || '112',
        countdownSeconds: state.user.countdownSeconds || 5
      });
      state.user = result.settings;
      setStatus($('theme-status'), 'Theme saved.', false);
    } catch (error) {
      setStatus($('theme-status'), error.message, true);
    }
  }

  async function changePassword(event) {
    event.preventDefault();
    const status = $('password-status');
    const current = $('current-password').value;
    const next = $('new-password').value;
    if (next.length < 8) {
      setStatus(status, 'Password must be at least 8 characters.', true);
      return;
    }
    if (next !== $('confirm-password').value) {
      setStatus(status, 'Passwords do not match.', true);
      return;
    }
    try {
      await postJSON('/api/v1/users/me/password', { currentPassword: current, newPassword: next });
      $('current-password').value = '';
      $('new-password').value = '';
      $('confirm-password').value = '';
      setStatus(status, 'Password changed.', false);
    } catch (error) {
      setStatus(status, error.message, true);
    }
  }

  async function deleteAccount() {
    const password = window.prompt('Confirm your password to delete your account. This cannot be undone.');
    if (password === null) return;
    try {
      await api('/api/v1/users/me', {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ password: password })
      });
      window.location.href = '/';
    } catch (error) {
      setStatus($('account-status'), error.message, true);
    }
  }

  /* ------------------------------------------------------------ pictures */

  function renderGallery(pictures) {
    const gallery = $('picture-gallery');
    gallery.innerHTML = '';
    pictures.forEach((picture) => {
      const figure = document.createElement('figure');
      figure.className = 'gallery-item' + (picture.primary ? ' is-primary' : '');
      figure.dataset.id = picture.id;

      const img = document.createElement('img');
      img.src = picture.thumbUrl;
      img.alt = 'Uploaded picture ' + picture.id;
      img.loading = 'lazy';
      img.addEventListener('click', () => openLightbox(picture.url));

      const caption = document.createElement('figcaption');

      const badge = document.createElement('span');
      badge.className = 'badge' + (picture.primary ? ' badge-primary' : '');
      badge.textContent = picture.primary ? 'Profile picture' : 'Picture';

      const primaryButton = document.createElement('button');
      primaryButton.type = 'button';
      primaryButton.className = 'link-button';
      primaryButton.dataset.action = 'primary';
      primaryButton.dataset.id = picture.id;
      primaryButton.textContent = 'Make profile picture';
      if (picture.primary) primaryButton.hidden = true;

      const deleteButton = document.createElement('button');
      deleteButton.type = 'button';
      deleteButton.className = 'link-button danger';
      deleteButton.dataset.action = 'delete';
      deleteButton.dataset.id = picture.id;
      deleteButton.textContent = 'Delete';

      caption.append(badge, primaryButton, deleteButton);
      figure.append(img, caption);
      gallery.appendChild(figure);
    });
    $('gallery-empty').hidden = pictures.length > 0;
  }

  async function uploadPicture(file) {
    const status = $('upload-status');
    if (!file) return;
    if (!/^image\/(png|jpeg)$/.test(file.type)) {
      setStatus(status, 'Please choose a JPG or PNG picture.', true);
      return;
    }
    if (file.size > 5 * 1024 * 1024) {
      setStatus(status, 'That picture is too large. Please choose a file under 5 MB.', true);
      return;
    }
    setStatus(status, 'Uploading...', false);

    const form = new FormData();
    form.append('file', file);

    try {
      const result = await api('/api/v1/users/me/pictures', { method: 'POST', body: form });
      state.pictures = result.pictures || [];
      renderGallery(state.pictures);
      const me = await api('/api/v1/auth/me');
      state.user = me.user;
      updateAvatarPreview();
      setStatus(status, 'Picture uploaded.', false);
      $('avatar-input').value = '';
    } catch (error) {
      setStatus(status, error.message, true);
    }
  }

  async function galleryAction(action, id) {
    const status = $('gallery-status');
    try {
      let result;
      if (action === 'primary') {
        result = await postJSON('/api/v1/users/me/pictures/' + id + '/primary', {});
      } else {
        result = await api('/api/v1/users/me/pictures/' + id, { method: 'DELETE' });
      }
      state.pictures = result.pictures || [];
      renderGallery(state.pictures);
      const me = await api('/api/v1/auth/me');
      state.user = me.user;
      updateAvatarPreview();
      setStatus(status, action === 'primary' ? 'Profile picture updated.' : 'Picture deleted.', false);
    } catch (error) {
      setStatus(status, error.message, true);
    }
  }

  function openLightbox(url) {
    if (!url) return;
    $('lightbox-image').src = url;
    $('lightbox').hidden = false;
    $('lightbox-close').focus();
  }

  function closeLightbox() {
    $('lightbox').hidden = true;
    $('lightbox-image').src = '';
  }

  /* ----------------------------------------------- moving picture area */

  function initPictureArea() {
    const area = $('picture-area');
    const slides = Array.prototype.slice.call(area.querySelectorAll('.slide'));
    const dotsBox = $('picture-dots');
    if (!slides.length) return;

    let index = 0;
    let timer = null;
    let paused = false;
    const interval = 6000;

    const dots = slides.map((slide, i) => {
      const dot = document.createElement('button');
      dot.type = 'button';
      dot.className = 'picture-dot' + (i === 0 ? ' is-active' : '');
      dot.setAttribute('aria-label', 'Show picture ' + (i + 1));
      dot.addEventListener('click', () => show(i));
      dotsBox.appendChild(dot);
      return dot;
    });

    function show(next) {
      index = (next + slides.length) % slides.length;
      slides.forEach((slide, i) => slide.classList.toggle('is-active', i === index));
      dots.forEach((dot, i) => dot.classList.toggle('is-active', i === index));
    }

    function start() {
      stop();
      if (motionEnabled()) {
        timer = setInterval(() => {
          if (!paused) show(index + 1);
        }, interval);
      }
    }

    function stop() {
      if (timer) clearInterval(timer);
      timer = null;
    }

    area.addEventListener('mouseenter', () => { paused = true; });
    area.addEventListener('mouseleave', () => { paused = false; });
    area.addEventListener('focusin', () => { paused = true; });
    area.addEventListener('focusout', () => { paused = false; });

    document.addEventListener('visibilitychange', () => {
      if (document.hidden) stop();
      else start();
    });

    start();
  }

  function motionEnabled() {
    const attribute = document.documentElement.getAttribute('data-motion');
    if (attribute === 'off') return false;
    const stored = (function () {
      try { return localStorage.getItem('victimcare.motion'); } catch (e) { return null; }
    })();
    if (stored === 'off') return false;
    return !window.matchMedia || !window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  }

  /* --------------------------------------------------------- resize bar */

  function initResize() {
    const control = $('app-size');
    const reset = $('reset-size');
    const original = 100;
    function apply(value) {
      const size = Math.max(85, Math.min(120, Number(value) || original));
      $('app-shell').style.width = size + '%';
      $('app-shell').style.marginLeft = ((100 - size) / 2) + '%';
      $('app-shell').style.marginRight = ((100 - size) / 2) + '%';
      control.value = size;
      control.setAttribute('aria-valuenow', String(size));
    }
    control.addEventListener('input', (event) => apply(event.target.value));
    reset.addEventListener('click', () => apply(original));
  }

  /* --------------------------------------------------------------- wire */

  function wire() {
    $('open-login').addEventListener('click', () => openAuth('login'));
    $('open-signup').addEventListener('click', () => openAuth('signup'));
    $('back-to-welcome').addEventListener('click', () => showScreen('welcome'));

    document.querySelectorAll('.password-toggle').forEach((button) => {
      button.addEventListener('click', () => {
        const input = $(button.dataset.passwordTarget);
        const visible = input.type === 'text';
        input.type = visible ? 'password' : 'text';
        button.setAttribute('aria-pressed', String(!visible));
        button.textContent = visible ? 'Show' : 'Hide';
      });
    });

    $('prototype-form').addEventListener('submit', submitAuth);
    $('signout').addEventListener('click', signOut);
    $('settings-signout').addEventListener('click', signOut);
    $('avatar-button').addEventListener('click', () => switchTab('settings'));

    $('create').addEventListener('click', () => { switchTab('report'); $('note').focus(); });
    $('tabs').addEventListener('click', (event) => {
      const button = event.target.closest('[data-tab]');
      if (button) switchTab(button.dataset.tab);
    });
    $('home-view').addEventListener('click', (event) => {
      const tile = event.target.closest('[data-go]');
      if (tile) switchTab(tile.dataset.go);
    });

    // The "back to home" button on every secondary screen.
    $('signed-in-view').addEventListener('click', (event) => {
      const button = event.target.closest('[data-go-home]');
      if (!button) return;
      switchTab('home');
      $('sos').focus();
    });

    document.querySelectorAll('[data-type]').forEach((button) => {
      button.addEventListener('click', () => {
        state.reportType = button.dataset.type;
        document.querySelectorAll('[data-type]').forEach((chip) => {
          chip.setAttribute('aria-pressed', String(chip === button));
        });
      });
    });

    $('send-report').addEventListener('click', async () => {
      const note = $('note').value.trim();
      $('note').value = '';
      state.practice = false;
      await prepareAlert(state.reportType, note);
    });

    $('sos').addEventListener('click', () => startCountdown(false));
    $('practice').addEventListener('click', () => startCountdown(true));
    $('cancel-alert').addEventListener('click', closeOverlay);
    $('close-alert').addEventListener('click', closeOverlay);
    $('copy-alert').addEventListener('click', copyAlertMessage);

    $('contact-form').addEventListener('submit', async (event) => {
      event.preventDefault();
      const status = $('contact-status');
      const id = parseInt($('contact-id').value, 10) || 0;
      try {
        const result = await postJSON('/api/v1/contacts', {
          id: id,
          name: $('contact-name').value.trim(),
          phone: $('contact-phone').value.trim()
        });
        state.contacts = result.contacts || [];
        state.contact = primaryContact() || {};
        renderContacts();
        resetContactForm();
        setStatus(status, id ? 'Contact updated.' : 'Contact saved.', false);
      } catch (error) {
        setStatus(status, error.message, true);
      }
    });

    $('cancel-contact-edit').addEventListener('click', () => {
      resetContactForm();
      setStatus($('contact-status'), '', false);
    });

    $('contact-list').addEventListener('click', (event) => {
      const button = event.target.closest('[data-contact-action]');
      if (!button) return;
      const action = button.dataset.contactAction;
      const id = parseInt(button.dataset.id, 10);
      if (action === 'edit') {
        $('contact-id').value = id;
        $('contact-name').value = button.dataset.name || '';
        $('contact-phone').value = button.dataset.phone || '';
        $('contact-form-title').textContent = 'Edit contact';
        $('cancel-contact-edit').hidden = false;
        $('contact-name').focus();
        return;
      }
      contactAction(action, id);
    });

    // Country and state dropdowns.
    [['signup-country', 'signup-state'], ['profile-country', 'profile-state']].forEach((pair) => {
      const countrySelect = $(pair[0]);
      const stateSelect = $(pair[1]);
      if (!countrySelect || !stateSelect) return;
      countrySelect.addEventListener('change', () => fillStates(countrySelect, stateSelect));
    });

    $('signup-password').addEventListener('input', updatePasswordRules);
    $('signup-password').addEventListener('blur', updatePasswordRules);

    $('profile-form').addEventListener('submit', saveProfile);
    $('settings-form').addEventListener('submit', saveSettings);
    $('password-form').addEventListener('submit', changePassword);
    $('delete-account').addEventListener('click', deleteAccount);

    $('clear-reports').addEventListener('click', async () => {
      try {
        const result = await api('/api/v1/incidents', { method: 'DELETE' });
        state.reports = result.incidents || [];
        renderReports();
        setStatus($('account-status'), 'Past reports cleared.', false);
      } catch (error) {
        setStatus($('account-status'), error.message, true);
      }
    });

    document.querySelectorAll('.theme-swatch').forEach((swatch) => {
      swatch.addEventListener('click', () => changeTheme(swatch.dataset.theme));
      swatch.addEventListener('keydown', (event) => {
        if (event.key !== 'ArrowRight' && event.key !== 'ArrowLeft') return;
        event.preventDefault();
        const swatches = Array.prototype.slice.call(document.querySelectorAll('.theme-swatch'));
        const current = swatches.indexOf(swatch);
        const next = (current + (event.key === 'ArrowRight' ? 1 : swatches.length - 1)) % swatches.length;
        swatches[next].focus();
        changeTheme(swatches[next].dataset.theme);
      });
    });

    $('setting-motion').addEventListener('change', (event) => {
      applyMotion(event.target.checked);
      setStatus($('theme-status'), event.target.checked ? 'Animations on.' : 'Animations off.', false);
    });

    $('avatar-input').addEventListener('change', (event) => uploadPicture(event.target.files[0]));
    $('picture-gallery').addEventListener('click', (event) => {
      const button = event.target.closest('[data-action]');
      if (button) galleryAction(button.dataset.action, button.dataset.id);
    });

    const preview = $('avatar-preview');
    if (preview) {
      preview.addEventListener('click', () => openLightbox(state.user.avatarUrl));
      preview.addEventListener('keydown', (event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault();
          openLightbox(state.user.avatarUrl);
        }
      });
    }
    $('lightbox-close').addEventListener('click', closeLightbox);
    $('lightbox').addEventListener('click', (event) => {
      if (event.target === $('lightbox')) closeLightbox();
    });

    document.querySelectorAll('[data-doc]').forEach((link) => {
      link.addEventListener('click', (event) => {
        event.preventDefault();
        setStatus($('about-note'), 'The ' + link.dataset.doc + ' document is part of the full release of this app.', false);
      });
    });

    document.addEventListener('keydown', (event) => {
      if (event.key === 'Escape') {
        if (!$('lightbox').hidden) closeLightbox();
        else if (!$('overlay').hidden) closeOverlay();
      }
    });
  }

  /* ------------------------------------------------- install and offline */

  function registerServiceWorker() {
    if (!('serviceWorker' in navigator)) return;
    window.addEventListener('load', () => {
      navigator.serviceWorker.register('/static/sw.js').catch(() => {
        /* the app still works without the offline cache */
      });
    });
  }

  function setupInstallPrompt() {
    const button = $('install-app');
    if (!button) return;
    let deferred = null;

    window.addEventListener('beforeinstallprompt', (event) => {
      event.preventDefault();
      deferred = event;
      button.hidden = false;
      setStatus($('install-note'), 'Victim Care can be installed on this device.', false);
    });

    button.addEventListener('click', async () => {
      if (!deferred) {
        setStatus($('install-note'), 'Use the browser menu and choose "Add to home screen".', false);
        return;
      }
      deferred.prompt();
      const choice = await deferred.userChoice;
      if (choice.outcome === 'accepted') {
        button.hidden = true;
        setStatus($('install-note'), 'Victim Care was installed.', false);
      }
      deferred = null;
    });

    window.addEventListener('appinstalled', () => {
      button.hidden = true;
      setStatus($('install-note'), 'Victim Care was installed.', false);
    });
  }

  /* --------------------------------------------------------------- boot */

  function boot() {
    // The deep link has to be read before the first tab switch replaces the
    // hash in the address bar.
    const wanted = (window.location.hash || '').replace('#', '');

    wire();
    initResize();
    initPictureArea();
    registerServiceWorker();
    setupInstallPrompt();

    if (state.signedIn) {
      applyMotion(state.user.motion !== false);
      state.countdownSeconds = state.user.countdownSeconds || 5;
      showScreen('app');
      switchTab('home');
      renderGallery(state.pictures);
      renderContacts();
      fillStates($('profile-country'), $('profile-state'), state.user.state || '');

      // Deep link: /#settings opens the settings tab straight away.
      if (views.indexOf(wanted) !== -1) switchTab(wanted);

      // Home screen shortcut: /?practice=1 runs a practice alert straight away.
      if (/[?&]practice=1/.test(window.location.search)) {
        window.setTimeout(function () { startCountdown(true); }, 600);
      }
    } else {
      showScreen('welcome');
      switchTab('home');
      updatePasswordRules();

      // Deep link: /#signup opens the sign up form straight away.
      if (wanted === 'signup' || wanted === 'login') openAuth(wanted);
    }
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', boot);
  } else {
    boot();
  }
})();
