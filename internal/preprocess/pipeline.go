package preprocess

import (
	"context"

	"llm-gateway/gateway/internal/providers"
)

type Pipeline interface {
	Run(ctx context.Context, req providers.ChatCompletionRequest) (Result, error)
}

type DefaultPipeline struct {
	normalizer Normalizer
	summarizer Summarizer
	classifier Classifier
}

func NewDefaultPipeline(normalizer Normalizer, summarizer Summarizer, classifier Classifier) *DefaultPipeline {
	if normalizer == nil {
		normalizer = NewNoopNormalizer()
	}
	if summarizer == nil {
		summarizer = NewNoopSummarizer()
	}
	if classifier == nil {
		classifier = NewNoopClassifier()
	}
	return &DefaultPipeline{
		normalizer: normalizer,
		summarizer: summarizer,
		classifier: classifier,
	}
}

func NewDefaultPipelineFromConfig(cfg Config, localProvider providers.Provider) *DefaultPipeline {
	var normalizer Normalizer = NewNoopNormalizer()
	if cfg.NormalizeEnabled {
		normalizer = NewLowRiskNormalizer("v1")
	}

	var summarizer Summarizer = NewNoopSummarizer()
	if cfg.SummaryEnabled {
		summarizer = NewModelBackedSummarizer(localProvider, cfg.SummaryModel, cfg.SummaryTriggerMessages, cfg.SummaryMaxRecentTurns, NewPlaceholderSummarizer(cfg.SummaryTriggerMessages, cfg.SummaryMaxRecentTurns))
	}

	var classifier Classifier = NewNoopClassifier()
	if cfg.ClassificationEnabled {
		classifier = NewModelBackedClassifier(localProvider, cfg.ClassifierModel, NewHeuristicClassifier())
	}

	return NewDefaultPipeline(normalizer, summarizer, classifier)
}

func NewNoopPipeline() *DefaultPipeline {
	return NewDefaultPipeline(nil, nil, nil)
}

func (p *DefaultPipeline) Run(ctx context.Context, req providers.ChatCompletionRequest) (Result, error) {
	result := Result{
		OriginalRequest:  req,
		ProcessedRequest: req,
	}

	processed, normalizeMeta, err := p.normalizer.Apply(ctx, result.ProcessedRequest)
	if err != nil {
		return Result{}, err
	}
	result.ProcessedRequest = processed
	result.Normalize = normalizeMeta

	processed, summaryMeta, err := p.summarizer.Apply(ctx, result.ProcessedRequest)
	if err != nil {
		return Result{}, err
	}
	result.ProcessedRequest = processed
	result.Summary = summaryMeta

	classificationMeta, err := p.classifier.Apply(ctx, result.ProcessedRequest)
	if err != nil {
		return Result{}, err
	}
	result.Classification = classificationMeta
	if classificationMeta.Applied {
		result.ProcessedRequest.TaskHint = classificationMeta.TaskHint
		result.ProcessedRequest.Complexity = classificationMeta.Complexity
		result.ProcessedRequest.ComplexityConfidence = classificationMeta.Confidence
	}

	return result, nil
}
