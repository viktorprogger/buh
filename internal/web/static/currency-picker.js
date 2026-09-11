(function () {
  function initPicker(picker) {
    if (picker.dataset.currencyPickerInitialized === 'true') return;
    picker.dataset.currencyPickerInitialized = 'true';

    var toggle = picker.querySelector('[data-currency-toggle]');
    var menu = picker.querySelector('[data-currency-menu]');
    var search = picker.querySelector('[data-currency-search]');
    var optionsBox = picker.querySelector('[data-currency-options]');
    var empty = picker.querySelector('[data-currency-empty]');
    var valueInput = picker.querySelector('[data-currency-value]');
    var label = picker.querySelector('[data-currency-label]');
    var options = Array.prototype.slice.call(picker.querySelectorAll('[data-currency-option]'));
    var activeIndex = 0;
    var visibleOptions = options.slice();
    var originalParent = menu.parentNode;
    var originalNextSibling = menu.nextSibling;
    var repositionFrame = null;
    var repositionListenersAttached = false;

    function reposition() {
      repositionFrame = null;
      if (menu.classList.contains('hidden')) return;

      var rect = toggle.getBoundingClientRect();
      var viewport = window.visualViewport;
      var viewportWidth = viewport ? viewport.width : document.documentElement.clientWidth;
      var viewportHeight = viewport ? viewport.height : document.documentElement.clientHeight;
      var viewportLeft = viewport ? viewport.offsetLeft : 0;
      var viewportTop = viewport ? viewport.offsetTop : 0;
      var viewportRight = viewportLeft + viewportWidth;
      var viewportBottom = viewportTop + viewportHeight;
      // getBoundingClientRect() is layout-viewport-relative on engines where
      // the visual viewport is offset. Fixed coordinates need that offset added
      // to stay aligned with the trigger while the user pans or zooms.
      var anchorLeft = rect.left + viewportLeft;
      var anchorTop = rect.top + viewportTop;
      var anchorBottom = rect.bottom + viewportTop;
      var gap = 4;
      var menuRect = menu.getBoundingClientRect();
      var availableBelow = viewportBottom - anchorBottom - gap;
      var availableAbove = anchorTop - viewportTop - gap;
      var height = Math.min(menuRect.height, Math.max(0, viewportHeight - gap * 2));
      var top = anchorBottom + gap;
      if (availableBelow < height && availableAbove > availableBelow) {
        top = anchorTop - height - gap;
      }
      top = Math.max(viewportTop + gap, Math.min(top, viewportBottom - height - gap));
      var width = Math.min(menuRect.width, Math.max(0, viewportWidth - gap * 2));
      var left = Math.max(viewportLeft + gap, Math.min(anchorLeft, viewportRight - width - gap));
      menu.style.boxSizing = 'border-box';
      menu.style.width = width + 'px';
      menu.style.left = left + 'px';
      menu.style.top = top + 'px';
      menu.style.maxHeight = Math.max(0, viewportHeight - gap * 2) + 'px';
      var searchHeight = search.getBoundingClientRect().height;
      optionsBox.style.maxHeight = Math.max(0, Math.min(192, viewportHeight - searchHeight - 20)) + 'px';
    }

    function scheduleReposition() {
      if (repositionFrame === null && !menu.classList.contains('hidden')) {
        repositionFrame = window.requestAnimationFrame(reposition);
      }
    }

    function addRepositionListeners() {
      if (repositionListenersAttached) return;
      repositionListenersAttached = true;
      window.addEventListener('scroll', scheduleReposition, true);
      window.addEventListener('resize', scheduleReposition);
      if (window.visualViewport) {
        window.visualViewport.addEventListener('resize', scheduleReposition);
        window.visualViewport.addEventListener('scroll', scheduleReposition);
      }
    }

    function removeRepositionListeners() {
      if (!repositionListenersAttached) return;
      repositionListenersAttached = false;
      window.removeEventListener('scroll', scheduleReposition, true);
      window.removeEventListener('resize', scheduleReposition);
      if (window.visualViewport) {
        window.visualViewport.removeEventListener('resize', scheduleReposition);
        window.visualViewport.removeEventListener('scroll', scheduleReposition);
      }
      if (repositionFrame !== null) {
        window.cancelAnimationFrame(repositionFrame);
        repositionFrame = null;
      }
    }

    function setActive(index, scroll) {
      if (!visibleOptions.length) {
        activeIndex = -1;
      } else {
        activeIndex = (index + visibleOptions.length) % visibleOptions.length;
      }
      options.forEach(function (option) {
        var active = visibleOptions[activeIndex] === option;
        option.toggleAttribute('data-active', active);
        option.classList.toggle('bg-primary-light', active);
        option.classList.toggle('ring-1', active);
        option.classList.toggle('ring-primary', active);
      });
      if (search) {
        search.setAttribute('aria-activedescendant', activeIndex >= 0 ? visibleOptions[activeIndex].id : '');
      }
      if (scroll && activeIndex >= 0) {
        visibleOptions[activeIndex].scrollIntoView({block: 'nearest'});
      }
    }

    function filterOptions(resetPosition) {
      var query = (search.value || '').trim().toUpperCase();
      visibleOptions = options.filter(function (option) {
        return query === '' || option.dataset.currency.toUpperCase() === query;
      });
      options.forEach(function (option) {
        option.classList.toggle('hidden', visibleOptions.indexOf(option) === -1);
      });
      empty.classList.toggle('hidden', visibleOptions.length !== 0);
      if (resetPosition) optionsBox.scrollTop = 0;
      setActive(0, false);
    }

    function onDocumentClick(event) {
      if (!picker.contains(event.target) && !menu.contains(event.target)) close(false);
    }

    function open() {
      menu.classList.remove('hidden');
      // Portal the menu out of clipping table/KPO overflow containers. Keep the
      // original position so HTMX and form markup are unchanged after closing.
      document.body.appendChild(menu);
      menu.style.position = 'fixed';
      toggle.setAttribute('aria-expanded', 'true');
      search.setAttribute('aria-expanded', 'true');
      document.addEventListener('click', onDocumentClick);
      filterOptions(false);
      addRepositionListeners();
      scheduleReposition();
      if (activeIndex >= 0) visibleOptions[activeIndex].scrollIntoView({block: 'nearest'});
      search.focus();
    }

    function close(returnFocus) {
      removeRepositionListeners();
      document.removeEventListener('click', onDocumentClick);
      menu.classList.add('hidden');
      menu.style.position = '';
      menu.style.left = '';
      menu.style.top = '';
      menu.style.maxHeight = '';
      menu.style.width = '';
      menu.style.boxSizing = '';
      optionsBox.style.maxHeight = '';
      if (originalParent) {
        originalParent.insertBefore(menu, originalNextSibling && originalNextSibling.parentNode === originalParent ? originalNextSibling : null);
      }
      toggle.setAttribute('aria-expanded', 'false');
      search.setAttribute('aria-expanded', 'false');
      if (returnFocus) toggle.focus();
    }

    function cleanup() {
      close(false);
      picker._currencyPickerCleanup = null;
    }

    function choose(option) {
      if (!option) return;
      var currency = option.dataset.currency;
      valueInput.value = currency;
      label.textContent = currency;
      options.forEach(function (item) {
        item.setAttribute('aria-selected', item === option ? 'true' : 'false');
      });
      close(true);
      picker.dispatchEvent(new CustomEvent('currency-change', {bubbles: true, detail: {currency: currency}}));
    }

    toggle.addEventListener('click', function () {
      if (menu.classList.contains('hidden')) open();
      else close(false);
    });

    search.addEventListener('input', function () {
      // Editing the query, rather than merely pressing a control, resets the
      // active option and the scroll position to the first visible currency.
      filterOptions(true);
    });

    search.addEventListener('keydown', function (event) {
      if (event.key === 'ArrowDown') {
        event.preventDefault();
        setActive(activeIndex + 1, true);
      } else if (event.key === 'ArrowUp') {
        event.preventDefault();
        setActive(activeIndex - 1, true);
      } else if (event.key === 'Enter') {
        event.preventDefault();
        if (activeIndex >= 0) choose(visibleOptions[activeIndex]);
      } else if (event.key === 'Escape') {
        event.preventDefault();
        close(true);
      }
    });

    options.forEach(function (option) {
      option.addEventListener('click', function () { choose(option); });
      option.addEventListener('mouseenter', function () {
        var index = visibleOptions.indexOf(option);
        if (index >= 0) setActive(index, false);
      });
    });

    picker._currencyPickerCleanup = cleanup;
    toggle.setAttribute('aria-expanded', 'false');
    search.setAttribute('aria-expanded', 'false');
    filterOptions(true);
  }

  function initAll(root) {
    root = root || document;
    if (root.matches && root.matches('[data-currency-picker]')) initPicker(root);
    root.querySelectorAll('[data-currency-picker]').forEach(initPicker);
  }

  document.addEventListener('htmx:beforeCleanupElement', function (event) {
    var elt = event.detail && event.detail.elt;
    if (!elt) return;
    document.querySelectorAll('[data-currency-picker]').forEach(function (picker) {
      if ((elt === picker || elt.contains(picker)) && picker._currencyPickerCleanup) {
        picker._currencyPickerCleanup();
      }
    });
  });
  document.addEventListener('DOMContentLoaded', function () { initAll(document); });
  document.addEventListener('htmx:load', function (event) { initAll(event.detail.elt); });
})();
