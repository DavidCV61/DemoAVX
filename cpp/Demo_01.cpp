#include "Demo_01.h"
#include "ImageMatrix.h"
#include <iomanip>
#include <iostream>
#include <stdexcept>

using namespace std;

const char *c_TestImageFileName = "ImageA.png";

static void ProcessImage(void);

int main() {
  try {
    ProcessImage();
    ProcessImage_bm();
  }

  catch (exception &ex) {
    cout << "Demo_01 exception: " << ex.what() << '\n';
  }
}

static void ProcessImage(void) {
  const char nl = '\n';
  const char *fn_mask0 = "Demo_01_ProcessImage_Mask0.png";
  const char *fn_mask1 = "Demo_01_ProcessImage_Mask1.png";

  ImageMatrix im_src(c_TestImageFileName, PixelType::Gray8);
  size_t im_h = im_src.GetHeight();
  size_t im_w = im_src.GetWidth();
  ImageMatrix im_mask0(im_h, im_w, PixelType::Gray8);
  ImageMatrix im_mask1(im_h, im_w, PixelType::Gray8);

  ITD itd0;
  itd0.m_PbSrc = im_src.GetPixelBuffer<uint8_t>();
  itd0.m_PbMask = im_mask0.GetPixelBuffer<uint8_t>();
  itd0.m_NumPixels = im_src.GetNumPixels();
  itd0.m_Threshold = c_TestThreshold;

  ITD itd1;
  itd1.m_PbSrc = im_src.GetPixelBuffer<uint8_t>();
  itd1.m_PbMask = im_mask1.GetPixelBuffer<uint8_t>();
  itd1.m_NumPixels = im_src.GetNumPixels();
  itd1.m_Threshold = c_TestThreshold;

  // Threshold image
  ThresholdImage_Cpp(&itd0);
  ThresholdImage_Iavx2(&itd1);
  im_mask0.SaveImage(fn_mask0, ImageFileType::PNG);
  im_mask1.SaveImage(fn_mask1, ImageFileType::PNG);

  // Calculate mean of masked pixels
  CalcImageMean_Cpp(&itd0);
  CalcImageMean_Iavx2(&itd1);

  const unsigned int w = 12;
  cout << fixed << setprecision(4);
  cout << "\nResults for ProcessImage() using file ";
  cout << c_TestImageFileName << nl << nl;
  cout << "                            Cpp         Iavx2\n";
  cout << "---------------------------------------------\n";
  cout << "SumPixelsMasked:   ";
  cout << setw(w) << itd0.m_SumMaskedPixels << "  ";
  cout << setw(w) << itd1.m_SumMaskedPixels << nl;
  cout << "NumPixelsMasked:   ";
  cout << setw(w) << itd0.m_NumMaskedPixels << "  ";
  cout << setw(w) << itd1.m_NumMaskedPixels << nl;
  cout << "MeanMaskedPixels:  ";
  cout << setw(w) << itd0.m_MeanMaskedPixels << "  ";
  cout << setw(w) << itd1.m_MeanMaskedPixels << nl;
}
