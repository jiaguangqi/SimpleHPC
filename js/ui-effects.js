(function () {
  'use strict';

  const root = document.documentElement;
  const body = document.body;
  const reduceMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches;

  if (!body?.classList.contains('shpc-sample-page')) return;

  document.addEventListener('DOMContentLoaded', () => {
    document.querySelector('main.main')?.classList.add('shpc-page-fade');
  });

  if (!reduceMotion) {
    let pending = false;
    window.addEventListener('pointermove', event => {
      if (pending) return;
      pending = true;
      window.requestAnimationFrame(() => {
        root.style.setProperty('--fx-mouse-x', `${Math.round((event.clientX / window.innerWidth) * 100)}%`);
        root.style.setProperty('--fx-mouse-y', `${Math.round((event.clientY / window.innerHeight) * 100)}%`);
        pending = false;
      });
    }, {passive: true});
  }

}());
