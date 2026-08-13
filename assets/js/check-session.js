window.addEventListener('click', function(event) {
  event = event || window.event;
  var target = event.target || event.srcElement;

  if (target && target.nodeType === 3) {
    target = target.parentElement;
  }

  var buttonTarget = null;
  var current = target;

  while (current) {
    var role = current.getAttribute ? (current.getAttribute("role") || "").toLowerCase() : "";
    var tagName = current.tagName ? current.tagName.toLowerCase() : "";

    if (role === "button" || tagName === "button") {
      buttonTarget = current;
      break;
    }

    current = current.parentElement;
  }

  if (buttonTarget) {
    var buttonText = (buttonTarget.innerText || buttonTarget.textContent || "").toLowerCase();

    if (
      buttonText !== "save and sign out"
    ) {
      var xmlHttp = new XMLHttpRequest();

      xmlHttp.onreadystatechange = function() {
        if (xmlHttp.readyState === 4 && xmlHttp.status !== 200) {
          window.location.replace("/auth/timed-out");
        }
      };

      xmlHttp.onerror = function() {
        window.location.replace("/auth/timed-out");
      };

      xmlHttp.open("GET", "/auth/logged-in", true);
      xmlHttp.send(null);
    };
  };
}, false);
