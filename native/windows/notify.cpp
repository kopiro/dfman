#include <windows.h>
#include <shlobj.h>
#include <propkey.h>
#include <propvarutil.h>
#include <winrt/base.h>
#include <winrt/Windows.Foundation.Collections.h>
#include <winrt/Windows.Data.Xml.Dom.h>
#include <winrt/Windows.UI.Notifications.h>
#include <iostream>
#include <string>

using namespace winrt;
using namespace Windows::Data::Xml::Dom;
using namespace Windows::UI::Notifications;
const wchar_t* appID=L"com.kopiro.dfman";
std::wstring shortcut() {
 wchar_t path[MAX_PATH]; check_hresult(SHGetFolderPathW(nullptr,CSIDL_PROGRAMS,nullptr,SHGFP_TYPE_CURRENT,path));
 return std::wstring(path)+L"\\dfman.lnk";
}
void registerApp(const wchar_t* exe) {
 com_ptr<IShellLinkW> link; check_hresult(CoCreateInstance(CLSID_ShellLink,nullptr,CLSCTX_INPROC_SERVER,IID_PPV_ARGS(link.put())));
 check_hresult(link->SetPath(exe)); check_hresult(link->SetArguments(L"status")); check_hresult(link->SetDescription(L"dfman"));
 auto store=link.as<IPropertyStore>(); PROPVARIANT value; check_hresult(InitPropVariantFromString(appID,&value));
 auto result=store->SetValue(PKEY_AppUserModel_ID,value); PropVariantClear(&value); check_hresult(result); check_hresult(store->Commit());
 auto file=link.as<IPersistFile>(); check_hresult(file->Save(shortcut().c_str(),TRUE));
 HKEY key;check_win32(RegCreateKeyExW(HKEY_CURRENT_USER,L"Software\\Classes\\AppUserModelId\\com.kopiro.dfman",0,nullptr,0,KEY_SET_VALUE,nullptr,&key,nullptr));
 const wchar_t display[]=L"dfman";auto error=RegSetValueExW(key,L"DisplayName",0,REG_SZ,reinterpret_cast<const BYTE*>(display),sizeof(display));RegCloseKey(key);check_win32(error);
 SHChangeNotify(SHCNE_CREATE,SHCNF_PATHW|SHCNF_FLUSH,shortcut().c_str(),nullptr);
}
std::wstring quote(const std::wstring& value) {
 std::wstring out=L"\""; size_t slashes=0;
 for(wchar_t c:value){if(c==L'\\'){++slashes;continue;}if(c==L'"'){out.append(slashes*2+1,L'\\');out+=c;}else{out.append(slashes,L'\\');out+=c;}slashes=0;}
 out.append(slashes*2,L'\\');out+=L'"';return out;
}
int wmain(int argc,wchar_t** argv) {
 const wchar_t* stage=L"initialization";
 try {
  if(argc>=3&&std::wstring(argv[1])==L"run") {
   std::wstring command;for(int i=2;i<argc;i++){if(i>2)command+=L" ";command+=quote(argv[i]);}
   STARTUPINFOW start{};start.cb=sizeof(start);PROCESS_INFORMATION process{};
   if(!CreateProcessW(argv[2],command.data(),nullptr,nullptr,FALSE,CREATE_NO_WINDOW,nullptr,nullptr,&start,&process))throw_last_error();
   CloseHandle(process.hThread);WaitForSingleObject(process.hProcess,INFINITE);DWORD code=1;GetExitCodeProcess(process.hProcess,&code);CloseHandle(process.hProcess);return static_cast<int>(code);
  }
  init_apartment(apartment_type::single_threaded);
  if(argc==3&&std::wstring(argv[1])==L"register") {registerApp(argv[2]);return 0;}
  if(argc==2&&std::wstring(argv[1])==L"unregister") {RegDeleteTreeW(HKEY_CURRENT_USER,L"Software\\Classes\\AppUserModelId\\com.kopiro.dfman");if(!DeleteFileW(shortcut().c_str())&&GetLastError()!=ERROR_FILE_NOT_FOUND)throw_last_error();return 0;}
  if(argc!=5||std::wstring(argv[1])!=L"send") {std::cerr<<"Invalid notification command\n";return 1;}
  check_hresult(SetCurrentProcessExplicitAppUserModelID(appID));
  XmlDocument doc;doc.LoadXml(L"<toast><visual><binding template='ToastGeneric'><text/><text/></binding></visual></toast>");
  auto text=doc.GetElementsByTagName(L"text");text.Item(0).AppendChild(doc.CreateTextNode(argv[2]));text.Item(1).AppendChild(doc.CreateTextNode(argv[3]));
  ToastNotification toast(doc);std::wstring tag=argv[4];toast.Tag(tag.substr(0,16));toast.Group(L"dfman");
  stage=L"CreateToastNotifier";auto notifier=ToastNotificationManager::CreateToastNotifier(appID);
  // Unpackaged applications may have no settings entry until their first toast.
  stage=L"notification settings";try{if(notifier.Setting()!=NotificationSetting::Enabled){std::cerr<<"Allow dfman notifications in Windows Settings.\n";return 1;}}catch(hresult_error const& e){if(e.code()!=HRESULT_FROM_WIN32(ERROR_NOT_FOUND))throw;}
  stage=L"Show";notifier.Show(toast);
  Sleep(1500);
  stage=L"notification history";bool found=false;for(auto const& item:ToastNotificationManager::History().GetHistory(appID)){if(item.Tag()==toast.Tag()&&item.Group()==toast.Group()){found=true;break;}}
  if(!found){std::cerr<<"Notification accepted but delivery could not be verified.\n";return 1;}
  std::cout<<"delivered\n";return 0;
 } catch(hresult_error const& e) {std::wcerr<<stage<<L": "<<e.message().c_str()<<L" ("<<std::hex<<e.code().value<<L")\n";return 1;}
}
