# Referenced from https://docs.nvidia.com/deeplearning/nemo/user-guide/docs/en/main/starthere/intro.html

# Import NeMo's ASR, NLP and TTS collections
import nemo.collections.asr as nemo_asr
import nemo.collections.nlp as nemo_nlp
import nemo.collections.tts as nemo_tts

# Download an audio file that we will transcribe, translate, and convert the written translation to speech
import wget
wget.download("https://nemo-public.s3.us-east-2.amazonaws.com/zh-samples/common_voice_zh-CN_21347786.mp3")

# Instantiate a Mandarin speech recognition model and transcribe an audio file.
asr_model = nemo_asr.models.ASRModel.from_pretrained(model_name="stt_zh_citrinet_1024_gamma_0_25")
mandarin_text = asr_model.transcribe(['common_voice_zh-CN_21347786.mp3'])
print(mandarin_text)

# Instantiate Neural Machine Translation model and translate the text
nmt_model = nemo_nlp.models.MTEncDecModel.from_pretrained(model_name="nmt_zh_en_transformer24x6")
english_text = nmt_model.translate(mandarin_text)
print(english_text)

# Instantiate a spectrogram generator (which converts text -> spectrogram)
# and vocoder model (which converts spectrogram -> audio waveform)
spectrogram_generator = nemo_tts.models.FastPitchModel.from_pretrained(model_name="tts_en_fastpitch")
vocoder = nemo_tts.models.HifiGanModel.from_pretrained(model_name="tts_en_hifigan")

# Parse the text input, generate the spectrogram, and convert it to audio
parsed_text = spectrogram_generator.parse(english_text[0])
spectrogram = spectrogram_generator.generate_spectrogram(tokens=parsed_text)
audio = vocoder.convert_spectrogram_to_audio(spec=spectrogram)

# Save the audio to a file
import soundfile as sf
sf.write("output_audio.wav", audio.to('cpu').detach().numpy()[0], 22050)
