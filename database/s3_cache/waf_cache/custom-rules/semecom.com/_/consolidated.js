
export default {
    "name": "semecom.com-@-consolidated",
    "domain": "semecom.com",
    "slug": "@",
    "code": "\n// Rule: test (Slug: @)\nif (req.url.includes(\"block=true\")) {\n  return helpers.block();\n}\n\n\n// Rule: aaa (Slug: @)\nif (req.url.includes(\"block=true\")) { return helpers.block(); }\n\n\n// Rule: aaa (Slug: @)\nif (req.url.includes(\"block=true\")) { return helpers.block(); }\n\n",
    "description": "Consolidated WAF rules for semecom.com/@. Total rules: 3"
};
