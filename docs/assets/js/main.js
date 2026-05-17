(function () {
  // Resources dropdown
  const dropdown = document.getElementById('nav-resources');
  const trigger = dropdown && dropdown.querySelector('.nav-dropdown-trigger');

  function openDropdown() {
    dropdown.setAttribute('data-open', '');
    trigger.setAttribute('aria-expanded', 'true');
  }

  function closeDropdown() {
    dropdown.removeAttribute('data-open');
    trigger && trigger.setAttribute('aria-expanded', 'false');
  }

  trigger && trigger.addEventListener('click', function (e) {
    e.stopPropagation();
    dropdown.hasAttribute('data-open') ? closeDropdown() : openDropdown();
  });

  document.addEventListener('click', function (e) {
    if (dropdown && !dropdown.contains(e.target)) closeDropdown();
  });

  document.addEventListener('keydown', function (e) {
    if (e.key === 'Escape') closeDropdown();
  });

  // Mobile sidebar toggle
  const sidebar = document.getElementById('docs-sidebar');
  const overlay = document.getElementById('sidebar-overlay');
  const hamburger = document.getElementById('hamburger');

  function openSidebar() {
    sidebar && sidebar.classList.add('open');
    overlay && overlay.classList.add('open');
  }

  function closeSidebar() {
    sidebar && sidebar.classList.remove('open');
    overlay && overlay.classList.remove('open');
  }

  hamburger && hamburger.addEventListener('click', function () {
    sidebar && sidebar.classList.contains('open') ? closeSidebar() : openSidebar();
  });

  overlay && overlay.addEventListener('click', closeSidebar);

  // Active nav state — match current path to sidebar links
  const navLinks = document.querySelectorAll('.sidebar-nav a');
  const currentPath = window.location.pathname.replace(/\/$/, '');

  navLinks.forEach(function (link) {
    const linkPath = link.getAttribute('href').replace(/\/$/, '');
    if (currentPath === linkPath || currentPath.endsWith(linkPath)) {
      link.classList.add('active');
    }
  });
})();
