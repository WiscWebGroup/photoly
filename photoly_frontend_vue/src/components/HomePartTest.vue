<script setup>
import { RouterLink, RouterView } from 'vue-router'
import router from "../router/index.js";
import axios from "axios";
import {Folder28Regular, Archive16Regular as ArchiveIcon, FolderAdd20Regular, FolderArrowRight20Regular, ArrowUpload20Filled,
  ArrowDownload20Filled, Eye20Regular, Delete20Regular, Edit20Regular } from "@vicons/fluent"
import { ref, defineComponent, nextTick, h } from 'vue';
import { NTag } from "naive-ui";
import { useMessage, NIcon } from 'naive-ui'
import MoveToShowFolder from "./MoveToShowFolder.vue"
import L from "leaflet";
import 'leaflet/dist/leaflet.css';
import MediaList from '@/components/MediaList.vue'

</script>

<template>
  <div>

    <!-- This part is for the navigation bar -->

    <n-back-top :right="100" />
    <n-breadcrumb separator=">">
      <n-icon size="25" :depth="3" style="vertical-align:top">
        <svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 24 24"><g fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M5 7l5 5l-5 5"></path><path d="M13 17h6"></path></g>
        </svg>
      </n-icon>
      <n-breadcrumb-item v-for="pathItem in nsPath" v-on:click="setCurrNsId(pathItem.nsId)">
        <n-icon /> {{ pathItem.nsName }}</n-breadcrumb-item>
    </n-breadcrumb>

    <n-divider />

    <!-- This part is for folder and medias -->
    <n-space v-show="nsId === -1">
          <n-spin size="large" />
    </n-space>
    <div @contextmenu="blankSpaceMenu" v-show="nsId !== -1">

      <!-- This part is for loading folders' buttons into the "folders" section -->
      
      <div>
        <n-space style="align-items:baseline">
          <n-h2>Folders</n-h2>
          <n-switch v-model:value="folderMultiSelectMode" >
            <template #checked>
              Folder Select Mode
            </template>
            <template #unchecked>
              Select Mode Off
            </template>
          </n-switch>
          <n-button round style="width: 5rem" v-show="folderMultiSelectMode" @click="clearCheckboxFolder">
              Clear
          </n-button>
          <n-button round style="width: 7rem" v-show="folderMultiSelectMode" @click="selectAllCheckboxFolder">
              Select All
          </n-button>
          <n-button round style="width: 7rem" v-show="folderMultiSelectMode" @click="moveSelectedFolder" type="info">
              Move
          </n-button>
          <n-button round style="width: 10rem" v-show="folderMultiSelectMode" @click="() => {showFolderMultiDeleteModal = true;}" type="error">
              Delete Selected
          </n-button>
        </n-space>
        <n-space>
          <n-button secondary strong size="large" v-for="folder in nsChildren" @click.left.native="handleFolderMulti(folder.nsId, $event)"
           @click.right.native="folderSpaceMenu(folder.nsId, folder.nsName, $event)">
            <template #icon>
              <n-icon>
                <Folder28Regular />
              </n-icon>
            </template>
            {{ folder.nsName.length <= 14 ? folder.nsName : folder.nsName.substring(0, 12) + "..." }}
            <n-checkbox size="large" 
                 style="margin-left: 0.5rem;" @update:checked="() => {folder.checked = !folder.checked;}"
                  v-show="folderMultiSelectMode" class="folderChecks" :checked="folder.checked"/>
          </n-button>
        </n-space>
      </div>
      <!-- ENDOF: This part is for loading folders' buttons into the "folders" section -->

      <!-- This part is for loading photos cards into the "media" section -->

      <MediaList @queryPhotos="queryPhotos" ref="mediaListRef" :nsOrNot="true"/>
      <!-- ENDOF: This part is for loading photos cards into the "media" section -->

    </div>
    <!-- ENDOF: This part is for folder and medias -->

    <!-- MENU PART -->
    <!-- This part is for redesign of right click menu for folder, media, and blank space -->
    <n-dropdown
        placement="bottom-start"
        trigger="manual"
        :x="mouseX"
        :y="mouseY"
        :options="blankSpaceMenuOptions"
        :show="showBlankSpaceMenu"
        :on-clickoutside="onClickOutsideBlank"
        @select="handleSelectBlank"
      />
      <n-dropdown
        placement="bottom-start"
        trigger="manual"
        :x="mouseX"
        :y="mouseY"
        :options="FolderSpaceMenuOptions"
        :show="showFolderSpaceMenu"
        :on-clickoutside="onClickOutsideFolder"
        @select="handleSelectFolder"
      />
      <!-- ENDOF: This part is for redesign of right click menu for folder, media, and blank space -->
      <!-- ENDOF: MENU PART -->

    <!-- MODAL PART -->

    <!-- This part is for modal of uploading photos to the current folder -->
    <n-modal v-model:show="showUploadPhotoModal">
      <n-card
        style="width: 400px; border-radius: 10px;"
        title="Upload Photo or Video"
        :bordered="false"
        size="huge"
        role="dialog"
        aria-modal="true"
      >
        <n-upload
            ref="fileUploader"
            multiple
            default-upload: false
            :custom-request="addPhotoToArea"
            :on-remove="removeItemFromUpload"
            :max="15"
            accept="image/*, video/*"
        >

          <n-upload-dragger>
            <div style="margin-bottom: 12px">
              <n-icon size="48" :depth="3">
                <archive-icon />
              </n-icon>
            </div>
            <n-text style="font-size: 16px">
              Click or Drag File to This Area
            </n-text>
          </n-upload-dragger>
        </n-upload>

        <n-button color="#74D2E7" round size="large" style="margin-top: 1rem; width: 100%;" @click="doUpload" :loading="uploadBtnLoading">
          Upload
        </n-button>

      </n-card>
    </n-modal>
    <!-- ENDOF: This part is for modal of uploading photos to the current folder -->

    <!-- This part is for modal of creating a new folder to the current folder -->
    <n-modal v-model:show="showCreateFolderModal">
      <n-card
        style="width: 400px; border-radius: 10px;"
        title="Create a New Folder"
        :bordered="false"
        size="huge"
        role="dialog"
        aria-modal="true"
      >
        <n-input type="text" round size="medium" placeholder="Folder Name" v-model:value="createFolderInputName" @keyup.enter="createFolder"  />

        <n-button color="#74D2E7" id="bt1" round size="large" style="margin-top: 1rem; width: 100%;" @click="createFolder">
          Create
        </n-button>

      </n-card>
    </n-modal>
    <!-- ENDOF: This part is for modal of creating a new folder to the current folder -->

    <!-- This part is for modal of confirming to delete a child folder -->
    <n-modal v-model:show="showDeleteFolderConfirmModal">
      <n-card
        style="width: 400px; border-radius: 10px;"
        title="Confirm to Delete"
        :bordered="false"
        size="huge"
        role="dialog"
        aria-modal="true"
      >
        Do you really want to DELETE this folder?
        <p>*All photos and folders within this folder will be destroied.</p>

        <n-space style="margin-top: 1rem">
          <n-button type="error" id="bt1" round size="large" style="margin-top: 1rem; width: 100%;" @click="deleteFolder">
            Delete
          </n-button>
          <n-button id="bt1" round size="large" style="margin-top: 1rem; width: 100%;" @click="() => {showDeleteFolderConfirmModal = false;}">
            Cancel
          </n-button>
        </n-space>

      </n-card>
    </n-modal>
    <!-- ENDOF: This part is for modal of confirming to delete a child folder -->

    <!-- This part is for modal of edit a folder's name -->
    <n-modal v-model:show="showEditFolderNameModal">
      <n-card
        style="width: 400px; border-radius: 10px;"
        title="Edit Folder's Name"
        :bordered="false"
        size="huge"
        role="dialog"
        aria-modal="true"
      >
        <n-input type="text" round size="medium" placeholder="Folder Name" v-model:value="editFolderInputName" @keyup.enter="updateFolderName"  />

        <n-button color="#74D2E7" id="bt1" round size="large" style="margin-top: 1rem; width: 100%;" @click="updateFolderName">
          Update Name
        </n-button>

      </n-card>
    </n-modal>
    <!-- ENDOF: This part is for modal of edit a folder's name -->

    <!-- This part is for modal of confirming to delete multiple folders -->
    <n-modal v-model:show="showFolderMultiDeleteModal">
      <n-card
        style="width: 400px; border-radius: 10px;"
        title="Confirm to Delete"
        :bordered="false"
        size="huge"
        role="dialog"
        aria-modal="true"
      >
        Do you really want to DELETE these folders?
        <p>*All photos and folders within this folder will be destroied.</p>

        <n-space style="margin-top: 1rem">
          <n-button type="error" id="bt1" round size="large" style="margin-top: 1rem; width: 100%;" @click="deleteSelectedFolder">
            Delete
          </n-button>
          <n-button id="bt1" round size="large" style="margin-top: 1rem; width: 100%;" @click="() => {showFolderMultiDeleteModal = false;}">
            Cancel
          </n-button>
        </n-space>

      </n-card>
    </n-modal>
    <!-- ENDOF: This part is for modal of confirming to delete multiple folders -->
    
    <!-- This part is for modal of moveTo Functions -->
    <MoveToShowFolder @moveToUpdate="moveItem" ref="moveToModuleRef" :originalNsId="moveToModalOriginalNsId" :disableOrgNsId="moveToModalDisableOrgNsId"/>
    <!-- ENDOF: This part is for modal of moveTo Functions -->

    <!-- This part is for modal of Multiple moveTo Functions -->
    <MoveToShowFolder @moveToUpdate="moveItemMulti" ref="moveToMultiModuleRef" :originalNsId="moveToModalOriginalNsId" :disableOrgNsId="moveToModalDisableOrgNsId"/>
    <!-- ENDOF: This part is for modal of Multiple moveTo Functions -->
    
    <!-- ENDOF: MODAL PART -->

  </div>

  
</template>

<script>

const moveToModuleRef = ref(null);
const moveToMultiModuleRef = ref(null);
const mediaListRef = ref(null);

export default defineComponent({
  data() {
    return {
      nsId: -1,
      parentNsId: -1,
      nsName: "",
      nsChildren: [],
      nsPath: [],
      nsPathStr: "",
      userToken: localStorage.getItem("HRD-Token"),
      showBlankSpaceMenu: false,
      showFolderSpaceMenu: false,
      showPhotoSpaceMenuPhotoInfo: null,
      mouseX: null,
      mouseY: null,
      blankSpaceMenuOptions: [
        {
          label: "Upload Photo",
          key: "upload",
          icon() {
            return h(NIcon, null, {
              default: () => h(ArrowUpload20Filled)
            });
          },
        },
        {
          label: "Create Folder",
          key: "create_folder",
          icon() {
            return h(NIcon, null, {
              default: () => h(FolderAdd20Regular)
            });
          },
        }
      ],
      FolderSpaceMenuOptions: [
        {
          label: "Delete",
          key: "delete",
          icon() {
            return h(NIcon, null, {
              default: () => h(Delete20Regular)
            });
          }
        },
        {
          label: "Rename",
          key: "rename",
          icon() {
            return h(NIcon, null, {
              default: () => h(Edit20Regular)
            });
          }
        },
        {
          label: "Move To",
          key: "move_to",
          icon() {
            return h(NIcon, null, {
              default: () => h(FolderArrowRight20Regular)
            });
          }
        }
      ],
      showUploadPhotoModal: false,
      uploadPhotoFiles: new FormData(),
      uploadBtnLoading: true,

      showCreateFolderModal: false,
      createFolderInputName: "",

      showDeleteFolderConfirmModal: false,
      showOperateFolderConfirmNsId: -1,

      showEditFolderNameModal: false,
      editFolderInputName: "",
      
      moveToModalOriginalNsId: -1,
      moveToModalDisableOrgNsId: false,
      
      folderMultiSelectMode: false,
      showFolderMultiDeleteModal: false,
    }
  },
  setup() {
    
    return {
      
    };
  },
  methods: {
    moveSelectedFolder() {
      // this.moveToModalOriginalNsId = this.showOperateFolderConfirmNsId;
      // this.moveToModalDisableOrgNsId = true;
      // this.openMoveToModalMulti();
      window.$message.warning("This is not implemented yet!");
    },
    deleteSelectedFolder() {
      this.nsChildren.forEach((ns) => {
        if (ns["checked"])
        {
          axios({
            method: 'post',
            baseURL: '',
            url: import.meta.env.VITE_APP_BASE_URL + "/namespace/delete?nsId=" + ns.nsId,
            headers: {
              "HRD-Token": localStorage.getItem("HRD-Token")
            },
            data: {
          }
          }).then((response) => {
            if (response.data.msgCode === 200)
            {
              window.$message.success("Folder deleted!");
              this.queryNsChildren();
            }else {
              window.$message.warning("Delete Error!")
            }
          })
          .catch(function (error) { // 请求失败处理
            window.$message.error(error);
          });
        }
      })
      
      this.clearCheckboxFolder();
      this.folderMultiSelectMode = false;
      this.showFolderMultiDeleteModal = false;
      
    },
    selectAllCheckboxFolder() {
      this.nsChildren.forEach((ns) => {
        ns["checked"] = true;
      })
    },
    clearCheckboxFolder() {
      this.nsChildren.forEach((ns) => {
        ns["checked"] = false;
      })
    },
    handleFolderMulti(folderId, event)
    {
      if (this.folderMultiSelectMode)
      {
        event.preventDefault();
        event.stopPropagation();
      }else {
        this.setCurrNsId(folderId);
      }
    },
    blankSpaceMenu (e) {
      e.preventDefault();
      e.stopPropagation();
      this.showBlankSpaceMenu = false;
      nextTick().then(() => {
        this.showBlankSpaceMenu = true;
        this.mouseX = e.clientX;
        this.mouseY = e.clientY;
        });
      
    },
    handleSelectBlank(key) {
      this.showBlankSpaceMenu = false;
      if (key === "upload")
      {
        this.uploadBtnLoading = false;
        this.showUploadPhotoModal = true;
      }
      if (key === "create_folder")
      {
        this.showCreateFolderModal = true;
      }
    },
    handleSelectFolder(key) {
      this.showFolderSpaceMenu = false;
      if (key === "delete")
      {
        this.showDeleteFolderConfirmModal = true;
      }
      if (key === "rename")
      {
        this.showEditFolderNameModal = true;
      }
      if (key === "move_to")
      {
        this.moveToModalOriginalNsId = this.showOperateFolderConfirmNsId;
        this.moveToModalDisableOrgNsId = true;
        this.openMoveToModal();
      }
    },
    onClickOutsideBlank () {
      this.showBlankSpaceMenu = false;
    },
    onClickOutsideFolder () {
      this.showFolderSpaceMenu = false;
    },
    folderSpaceMenu(childNsId, childFolderName, e) {
      e.preventDefault();
      e.stopPropagation();
      this.showFolderSpaceMenu = false;
      this.showOperateFolderConfirmNsId = childNsId;
      this.editFolderInputName = childFolderName;
      nextTick().then(() => {
        this.showFolderSpaceMenu = true;
        this.mouseX = e.clientX;
        this.mouseY = e.clientY;
        });
    },
    moveItemMulti (nsId) {
      // if it is true then this is a folder move
      if (this.moveToModalDisableOrgNsId)
      {
        
      }else {
        // else this is a photo move
        this.photoChildren.forEach((photo) => {
          if (photo["checked"])
          {
            axios({
              method: 'post',
              baseURL: '',
              url: import.meta.env.VITE_APP_BASE_URL + "/photo/changeNamespace?photoId=" + photo.photoId + "&nsId=" + nsId,
              headers: {
                "HRD-Token": localStorage.getItem("HRD-Token")
              },
              data: {
            }
            }).then((response) => {
              if (response.data.msgCode === 200)
              {
                window.$message.success("Media Location Updated!");
                this.queryPhotos();

              }else {
                window.$message.warning("Update Error!")
              }
            })
            .catch(function (error) { // 请求失败处理
              window.$message.error(error);
            });
          }
        })
        this.moveToModalOriginalNsId = -1;
        this.moveToModalDisableOrgNsId = false;

        this.photoMultiSelectMode = false;
        this.clearCheckboxPhoto();
        this.photoMultiSelectMode = false;
        
      }
      
    },
    moveItem (nsId) {
      // if it is true then this is a folder move
      if (this.moveToModalDisableOrgNsId)
      {
        axios({
          method: 'post',
          baseURL: '',
          url: import.meta.env.VITE_APP_BASE_URL + "/namespace/updateParent",
          headers: {
            "HRD-Token": localStorage.getItem("HRD-Token")
          },
          data: {
            "nsParentId": nsId,
            "nsId": this.showOperateFolderConfirmNsId
        }
        }).then((response) => {
          if (response.data.msgCode === 200)
          {
            this.queryNsChildren();
            window.$message.success("Folder Location Updated!");
            this.moveToModalOriginalNsId = -1;
            this.moveToModalDisableOrgNsId = false;
          }else {
            window.$message.warning("Update Error!")
          }
        })
        .catch(function (error) { // 请求失败处理
          window.$message.error(error);
        });
      }else {
        // else this is a photo move

        axios({
          method: 'post',
          baseURL: '',
          url: import.meta.env.VITE_APP_BASE_URL + "/photo/changeNamespace?photoId=" + this.showPhotoSpaceMenuPhotoInfo.photoId + "&nsId=" + nsId,
          headers: {
            "HRD-Token": localStorage.getItem("HRD-Token")
          },
          data: {
        }
        }).then((response) => {
          if (response.data.msgCode === 200)
          {
            this.queryPhotos();
            window.$message.success("Media Location Updated!");
            this.moveToModalOriginalNsId = -1;
            this.moveToModalDisableOrgNsId = false;
          }else {
            window.$message.warning("Update Error!")
          }
        })
        .catch(function (error) { // 请求失败处理
          window.$message.error(error);
        });
      }
      
    },
    openMoveToModal () {
      moveToModuleRef.value.openModal();
    },
    openMoveToModalMulti () {
      moveToMultiModuleRef.value.openModal();
    },
    updateFolderName () {
      if (this.editFolderInputName === "" || this.editFolderInputName === "root" || this.editFolderInputName === "/" || this.editFolderInputName === null)
      {
        window.$message.warning("Please input a valid folder name!")
      }else {
        axios({
          method: 'post',
          baseURL: '',
          url: import.meta.env.VITE_APP_BASE_URL + "/namespace/updateName",
          headers: {
            "HRD-Token": localStorage.getItem("HRD-Token")
          },
          data: {
            "nsName": this.editFolderInputName,
            "nsId": this.showOperateFolderConfirmNsId
        }
        }).then((response) => {
          if (response.data.msgCode === 200)
          {
            this.queryNsChildren();
            window.$message.success("Folder Name Updated!");
            this.showOperateFolderConfirmNsId = -1;
            this.editFolderInputName = "";
            this.showEditFolderNameModal = false;
          }else {
            window.$message.warning("Update Error!")
          }
        })
        .catch(function (error) { // 请求失败处理
          window.$message.error(error);
        });
      }
    },
    deleteFolder() {
      
      axios({
          method: 'post',
          baseURL: '',
          url: import.meta.env.VITE_APP_BASE_URL + "/namespace/delete?nsId=" + this.showOperateFolderConfirmNsId,
          headers: {
            "HRD-Token": localStorage.getItem("HRD-Token")
          },
          data: {
        }
        }).then((response) => {
          if (response.data.msgCode === 200)
          {
            this.queryNsChildren();
            window.$message.success("Folder deleted!");
            this.showOperateFolderConfirmNsId = -1;
            this.showDeleteFolderConfirmModal = false;
          }else {
            window.$message.warning("Delete Error!")
          }
        })
        .catch(function (error) { // 请求失败处理
          window.$message.error(error);
        });
    },
    createFolder() {
      if (this.createFolderInputName === "" || this.createFolderInputName === "root" || this.createFolderInputName === "/" || this.createFolderInputName === null)
      {
        window.$message.warning("Please input a valid folder name!")
      }else {
        axios({
          method: 'post',
          baseURL: '',
          url: import.meta.env.VITE_APP_BASE_URL + "/namespace/insert",
          headers: {
            "HRD-Token": localStorage.getItem("HRD-Token")
          },
          data: {
            "nsParentId": this.nsId,
            "nsName": this.createFolderInputName
        }
        }).then((response) => {
          if (response.data.msgCode === 200)
          {
            this.queryNsChildren();
            window.$message.success("Folder created!");
            this.createFolderInputName = "";
            this.showCreateFolderModal = false;
          }else {
            window.$message.warning("Creation Error!")
          }
        })
        .catch(function (error) { // 请求失败处理
          window.$message.error(error);
        });
      }
    },
    removeItemFromUpload(file)
    {
      var tempList = []
      this.uploadPhotoFiles.forEach((f) => {
        if (f.name !== file.file.name && f.size !== file.file.size)
        {
          tempList.push(f)
        }
      })
      this.uploadPhotoFiles = new FormData();
      for(var i = 0; i < tempList.length; i ++)
      {
        this.uploadPhotoFiles.append("files", tempList[i]);
      }
      
    },
    async doUpload() {
      this.uploadBtnLoading = true;
      if (this.uploadPhotoFiles.has("files"))
      {
        var tempList = []
        this.uploadPhotoFiles.forEach((f) => {
            tempList.push(f)
        })
        this.uploadPhotoFiles = new FormData();
        for(var i = 0; i < tempList.length; i ++)
        {
          this.uploadPhotoFiles.append("files", tempList[i]);
        }

        let photoList = [];
        this.uploadPhotoFiles.forEach((file) => {
          photoList.push({
            "nsId": this.nsId
          })
        })
        let photoStr = JSON.stringify(photoList);
        this.uploadPhotoFiles.append("photosStr", photoStr);
        
        await axios
          .post( import.meta.env.VITE_APP_BASE_URL + "/photo/inserts", this.uploadPhotoFiles, {
            headers: {
              'Content-Type': 'multipart/form-data',
              "HRD-Token": localStorage.getItem("HRD-Token")
            }
          })
          .then((response) => {
            if (response.data.msgCode === 200)
            {
              window.$message.success("Upload successful!");
              this.$refs['fileUploader'].clear();
              this.uploadPhotoFiles = new FormData();
              this.queryPhotos();
              this.showUploadPhotoModal = false;
              this.uploadBtnLoading = false;
            }else {
              window.$message.warning("Submit upload error!")
            }
            
          })
          .catch(function (error) {
            this.uploadBtnLoading = false;
            window.$message.warning(error)
          });
        
      }else {
        window.$message.warning("No Photo selected!");
        this.uploadBtnLoading = false;
      }
    },
    addPhotoToArea({file}) {
      this.uploadPhotoFiles.append("files", file.file);

    },
    setCurrNsId(nsId) {
      if (this.nsId !== nsId)
      {
        this.nsId = nsId;
        mediaListRef.value.updateNsId(nsId);
        this.update();
      }
      
    },
    trace() {
      axios({
        method: 'get',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/namespace/trace?nsId=" + this.nsId,
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
      }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          var infoT = response.data.t;
          this.nsPath = infoT;
          this.nsPathStr = "";
          for(var i = 0; i < infoT.length; i ++)
          {
            if (i == 0)
            {
              this.nsPathStr += "/root";
            }else {
              this.nsPathStr += "/" + infoT[i]["nsName"];
            }
          }
          if (infoT.length < 2)
          {
            this.parentNsId = -1;
          }else {
            this.parentNsId = infoT[infoT.length - 2]["nsId"];
          }
        }else {
          window.$message.error("trace Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        window.$message.error(error);
      });
    },
    queryNsChildren () {
      axios({
        method: 'get',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/namespace/getChildren?parentId=" + this.nsId,
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
      }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          var infoT = response.data.t;
          for(var i = 0; i < infoT.length; i ++)
          {
            infoT[i]["checked"] = false;
          }
          this.nsChildren = infoT
        }else {
          window.$message.error("query folder Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        // console.log(error);
        window.$message.error(error);
      });
    },
    queryPhotos() {
      axios({
        method: 'get',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/photo/getByNamespace?nsId=" + this.nsId,
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
      }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          var infoT = response.data.t;
          for(var i = 0; i < infoT.length; i ++)
          {
            infoT[i]["checked"] = false;
          }
          mediaListRef.value.updatePhotoList(infoT);
        }else {
          window.$message.error("query photo Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        console.log(error);
        window.$message.error(error);
      });
    },
    update() {
      this.queryPhotos();
      this.queryNsChildren();
      this.trace();
      mediaListRef.value.updateNsId(this.nsId);
    },
    queryRoot() {
      axios({
        method: 'get',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/namespace/getRoot/",
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
      }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          var infoT = response.data.t;
          this.nsId = infoT.nsId;
          this.nsName = infoT.nsName;
          this.update();
        }else {
          window.$message.error("get root folder Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        console.log(error);
        window.$message.error(error);
      });
    }
  },
  computed: {

  },
  mounted () {
    if (this.parentNsId === -1)
    {
      this.queryRoot();
    }
    window.$message = useMessage()
  },
  components: {

  }
})

</script>

<style scoped>
.n-button {
  border-radius: 10px;
  width: 15rem;
  
}
.cardImgDiv {
  width: 255px;
  height: 128px;
}
.cardImg {
  object-fit: cover;
  width: 100%;
  max-width: 100%;
  height: 128px;
  overflow: hidden;
}
.n-card {
  /*cursor: pointer;*/
  padding: 0;
}
</style>
