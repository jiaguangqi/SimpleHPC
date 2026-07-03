(function (root, factory) {
  var api = factory();
  if (typeof module === 'object' && module.exports) module.exports = api;
  root.JobPagination = api;
})(typeof globalThis !== 'undefined' ? globalThis : this, function () {
  function range(start, end) {
    var values = [];
    for (var value = start; value <= end; value++) values.push(value);
    return values;
  }

  function tokens(currentPage, totalPages) {
    totalPages = Math.max(1, Number(totalPages) || 1);
    currentPage = Math.max(1, Math.min(totalPages, Number(currentPage) || 1));
    if (totalPages <= 8) return range(1, totalPages);

    if (currentPage <= 5) {
      return range(1, 5).concat(['ellipsis'], range(totalPages - 1, totalPages));
    }
    if (currentPage >= totalPages - 2) {
      return range(1, 5).concat(['ellipsis'], range(totalPages - 2, totalPages));
    }
    return [1, 'ellipsis', currentPage - 1, currentPage, currentPage + 1, 'ellipsis', totalPages - 1, totalPages];
  }

  return { tokens: tokens };
});
