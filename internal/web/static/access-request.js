(function () {
  'use strict';

  var STORAGE_KEY = 'buh.access-request.v1';

  function readState() {
    var defaultState = {open: true, submitted: false, editing: false};
    try {
      var stored = JSON.parse(window.localStorage.getItem(STORAGE_KEY) || 'null');
      if (!stored || typeof stored !== 'object') return defaultState;
      return {
        open: typeof stored.open === 'boolean' ? stored.open : true,
        submitted: stored.submitted === true,
        editing: stored.editing === true,
      };
    } catch (_) {
      return defaultState;
    }
  }

  function writeState(state) {
    try {
      window.localStorage.setItem(STORAGE_KEY, JSON.stringify({
        open: state.open,
        submitted: state.submitted,
        editing: state.editing,
      }));
    } catch (_) {
      // The banner remains usable when storage is disabled or unavailable.
    }
  }

  function toggleHidden(element, hidden) {
    if (element) element.classList.toggle('hidden', hidden);
  }

  function applyState(elements, state, compact) {
    var success = state.submitted;

    toggleHidden(elements.successIcon, !success);
    toggleHidden(elements.defaultIcon, success);
    toggleHidden(elements.successTitle, !success);
    toggleHidden(elements.defaultTitle, success);
    elements.successBody.hidden = !success;
    elements.defaultBody.hidden = success;
    elements.another.hidden = !success || state.editing;
    elements.form.hidden = success && !state.editing;

    elements.summary.classList.toggle('bg-emerald-50', success);
    elements.summary.classList.toggle('hover:bg-emerald-100', success);
    elements.summary.classList.toggle('bg-blue-50', !success);
    elements.summary.classList.toggle('hover:bg-blue-100', !success);
    elements.summary.classList.toggle('px-3', compact);
    elements.summary.classList.toggle('py-2', compact);
    elements.summary.classList.toggle('px-5', !compact);
    elements.summary.classList.toggle('py-4', !compact);

    elements.successIcon.classList.toggle('h-6', compact);
    elements.successIcon.classList.toggle('w-6', compact);
    elements.successIcon.classList.toggle('h-10', !compact);
    elements.successIcon.classList.toggle('w-10', !compact);
    elements.successIcon.classList.toggle('text-xs', compact);
    elements.successIcon.classList.toggle('text-sm', !compact);

    elements.details.open = state.open;
  }

  function init() {
    var details = document.querySelector('[data-access-request]');
    if (!details) return;

    var elements = {
      details: details,
      summary: details.querySelector('[data-access-request-summary]'),
      successIcon: details.querySelector('[data-access-request-success-icon]'),
      defaultIcon: details.querySelector('[data-access-request-default-icon]'),
      successTitle: details.querySelector('[data-access-request-success-title]'),
      defaultTitle: details.querySelector('[data-access-request-default-title]'),
      successBody: details.querySelector('[data-access-request-success-body]'),
      defaultBody: details.querySelector('[data-access-request-default-body]'),
      another: details.querySelector('[data-access-request-another]'),
      form: details.querySelector('#access-request-form'),
      email: details.querySelector('#access-request-email'),
    };
    if (!elements.summary || !elements.form) return;

    var state = readState();
    var serverSuccess = details.dataset.serverSuccess === 'true';
    var newSubmission = serverSuccess && (!state.submitted || state.editing);

    if (serverSuccess) {
      state.submitted = true;
      state.editing = false;
      if (newSubmission) state.open = true;
    } else if (!state.submitted) {
      state.editing = false;
    }

    if (state.submitted && !state.editing && !newSubmission) {
      // A submitted request is intentionally quiet on later pages and reloads.
      state.open = false;
    }

    writeState(state);
    applyState(elements, state, state.submitted && !state.editing && !newSubmission);

    details.addEventListener('toggle', function () {
      state.open = details.open;
      writeState(state);
    });

    if (elements.another) {
      elements.another.addEventListener('click', function () {
        state.submitted = true;
        state.editing = true;
        state.open = true;
        writeState(state);
        applyState(elements, state, false);
        if (elements.email) elements.email.focus();
      });
    }
  }

  document.addEventListener('DOMContentLoaded', init);
})();
