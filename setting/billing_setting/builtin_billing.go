package billing_setting

// Built-in token prices use actual USD per million tokens. Keep new model
// defaults here instead of splitting them across the legacy ratio tables.
var builtinBillingExpr = map[string]string{
	// https://docs.x.ai/developers/models/grok-imagine-image-2.0
	// 按实际输出张数和输入图片数收费；乘 1000000 对齐表达式的金额单位。
	"grok-imagine-image-2.0": `(param("quality") == "medium" || (param("quality") != "low" && param("image_edit") == true))
		? (param("resolution") == "2k"
			? tier("2k_medium", (image_count * 0.08 + (param("input_image_count") ?? 0) * 0.01) * 1000000)
			: (param("resolution") == "1.5k"
				? tier("1.5k_medium", (image_count * 0.07 + (param("input_image_count") ?? 0) * 0.01) * 1000000)
				: tier("1k_medium", (image_count * 0.06 + (param("input_image_count") ?? 0) * 0.01) * 1000000)))
		: (param("resolution") == "2k"
			? tier("2k_low", (image_count * 0.06 + (param("input_image_count") ?? 0) * 0.01) * 1000000)
			: (param("resolution") == "1.5k"
				? tier("1.5k_low", (image_count * 0.05 + (param("input_image_count") ?? 0) * 0.01) * 1000000)
				: tier("1k_low", (image_count * 0.04 + (param("input_image_count") ?? 0) * 0.01) * 1000000)))`,
	// https://developers.openai.com/api/docs/pricing (Standard, 2026-09-09).
	// The Images API reports image output in output_tokens, normalized to c.
	"gpt-image-2":            `tier("standard", p * 5 + cr * 1.25 + img * 8 + img_cr * 2 + c * 30)`,
	"gpt-image-2.5-sunburst": `tier("standard", p * 5 + cr * 1.25 + img * 8 + img_cr * 2 + c * 30)`,
	"gpt-image-2.5-flare":    `tier("standard", p * 5 + cr * 1.25 + img * 8 + img_cr * 2 + c * 30)`,
	// https://developers.openai.com/api/docs/models/gpt-6-astra
	// Standard pricing; the long-context rates apply to the whole request.
	// Do not infer service-tier discounts from incoming request parameters:
	// channels filter service_tier by default, so it may not reach the upstream.
	"gpt-6-astra": `len <= 272000 ? tier("standard", p * 10 + c * 50 + cr * 1 + cc * 12.5) : tier("long_context", p * 20 + c * 75 + cr * 2 + cc * 25)`,
}
