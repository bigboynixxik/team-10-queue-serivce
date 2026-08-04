import * as z from "zod";

export const nullableString = z
	.string()
	.nullable()
	.optional()
	.transform((v) => v ?? "");

export const nullableNumber = z
	.number()
	.nullable()
	.optional()
	.transform((v) => v ?? 0);
