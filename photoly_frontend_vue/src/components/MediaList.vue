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

</script>

<template>
  <div>

      <!-- This part is for loading photos cards into the "media" section -->

      <div style="margin-top: 1rem; min-width: 90vh;">
        <n-space style="align-items:baseline">
          <n-h2>Medias</n-h2>
          <n-switch v-model:value="photoMultiSelectMode" >
            <template #checked>
              Photo Select Mode
            </template>
            <template #unchecked>
              Select Mode Off
            </template>
          </n-switch>
          <n-button round style="width: 5rem" v-show="photoMultiSelectMode" @click="clearCheckboxPhoto">
              Clear
          </n-button>
          <n-button round style="width: 7rem" v-show="photoMultiSelectMode" @click="selectAllCheckboxPhoto">
              Select All
          </n-button>
          <n-button round style="width: 7rem" v-show="photoMultiSelectMode" @click="moveSelectedPhoto" type="info">
              Move
          </n-button>
          <n-button round style="width: 10rem" v-show="photoMultiSelectMode" @click="() => {showPhotoMultiDeleteModal = true;}" type="error">
              Delete Selected
          </n-button>
        </n-space>
        
        <n-space>
          <div v-for="photo in photoChildren">
              <n-card style="border-radius: 20px; background-color: #f4f2f2;" v-show="photoMultiSelectMode">
                <n-checkbox size="large"
                 style="margin-left: 0.5rem;" @update:checked="() => {photo.checked = !photo.checked}" :checked="photo.checked"/>
              </n-card>
              <n-card style="border-radius: 20px;" @click.left.native="handlePhotoMulti(photo, $event)" hoverable @click.right.native="photoSpaceMenu(photo, $event)">
                <template #cover>
                  <div class="cardImgDiv" v-show="isphotoOrVideo(photo.format) === 1"><img v-bind:src="baseUThumbnail + userToken + '?photoId=' + photo.photoId" class="cardImg"></div>
                  <div class="cardImgDiv" v-show="isphotoOrVideo(photo.format) === 2"><img src="@/assets/icons/Video.png" class="cardImg"></div>
                  <div class="cardImgDiv" v-show="isphotoOrVideo(photo.format) === -1"><img src="@/assets/logo.ico" class="cardImg"></div>
                </template>
                <p>{{ photo.photoName.length <= 14 ? photo.photoName : photo.photoName.substring(0, 12) + "..." }}.{{ photo.format }}</p>
              </n-card>
          </div>
          
        </n-space>
      </div>
      <!-- ENDOF: This part is for loading photos cards into the "media" section -->

      <!-- This modal is for the photo detail after click on 'view' or open by left click the photo -->
      <n-modal v-model:show="showPhoto">
        <div style="min-width: 40vw ;max-width: 85vw; padding: 0; border-radius: 0.375rem;">
          <n-card
          style="padding: 0; border-radius: 0.375rem;"
            :bordered="false"
            size="huge"
            role="dialog"
            aria-modal="true"
          >
            <div style="display: flex; align-items: flex-start; flex-direction: row;">
              <img v-show="isphotoOrVideo(showPhotoInfo.format) === 1" v-bind:src="baseUPhoto + userToken + '?photoId=' + showPhotoInfo.photoId" 
              style="object-fit: fill; border-radius: 0.375rem; height: 100%; min-width: 30%; max-width: 75%;" @click.left.native="() => {showPhoto = false}"/>
              <video
                  v-show="isphotoOrVideo(showPhotoInfo.format) === 2"
                  :key="baseUVideo"
                  width="75%"
                  controls
                >
                  <source
                    :src="baseUVideo"
                    :type="'video/' + showPhotoInfo.format"
                  >
              </video>

              <div style="padding-left: 2rem;">
                <n-h2 >Metadata</n-h2>
                
                <n-collapse :default-expanded-names="['cm', 'lo']">
                  <n-collapse-item title="Common Metadata" name="cm">
                    <p>Name: {{ showPhotoInfo.photoName.length <= 14 ? showPhotoInfo.photoName : showPhotoInfo.photoName.substring(0, 12) + "..." }}.{{ showPhotoInfo.format }}</p>
                    <p>Upload Date: {{ showPhotoInfo.uploadDate }}</p>
                    <p>Format: {{ showPhotoInfo.format }}</p>
                    <p v-if="showPhotoExifInfo.File_Size">Size: {{ showPhotoExifInfo.File_Size }} </p>
                    <p>Path: {{ nsPathStr }}</p>
                    <p>Shared: {{ showPhotoInfo.visibility === 0 ? "No" : "Yes" }}</p>
                    <n-button strong secondary type="success" @click="download">
                      Download
                    </n-button>
                  </n-collapse-item>

                  <n-collapse-item title="Location" name="lo" v-show="showPhotoExifInfo.GPS_Latitude" display-directive="show">
                    <div id="img_map" style="height: 250px; width: 250px;"></div>
                  </n-collapse-item>

                  <n-collapse-item title="Exif Metadata" name="ex">
                    <p v-if="showPhotoExifInfo.Time">Time Taken: {{ showPhotoExifInfo.Time }} </p>
                    <p v-if="showPhotoExifInfo.Image_Height && showPhotoExifInfo.Image_Width">H/W: {{ showPhotoExifInfo.Image_Height ? showPhotoExifInfo.Image_Height.split(' ')[0] : showPhotoExifInfo.Image_Height }} * {{ showPhotoExifInfo.Image_Width }} </p>
                    <p v-if="showPhotoExifInfo.GPS_Latitude">GPS DD: {{ showPhotoExifInfo.GPS_Latitude }} {{ showPhotoExifInfo.GPS_Longitude }} </p>
                    <n-divider/>
                    <p v-if="showPhotoExifInfo.Model">Camera: {{ showPhotoExifInfo.Make }} {{ showPhotoExifInfo.Model }}</p>
                    <p v-if="showPhotoExifInfo.Lens_Make || showPhotoExifInfo.Lens_Model">Lens: {{ showPhotoExifInfo.Lens_Make }} {{ showPhotoExifInfo.Lens_Model }}</p>
                    <p v-if="showPhotoExifInfo.Focal_Length">Focal: {{ showPhotoExifInfo.Focal_Length }} </p>
                    <p v-if="showPhotoExifInfo.Aperture_Value">Aperture: {{ showPhotoExifInfo.Aperture_Value }} </p>
                    <p v-if="showPhotoExifInfo.Shutter_Speed_Value">Shutter: {{ showPhotoExifInfo.Shutter_Speed_Value }} </p>
                    <p v-if="showPhotoExifInfo.ISO_Speed_Ratings">ISO: {{ showPhotoExifInfo.ISO_Speed_Ratings }} </p>
                    <p v-if="showPhotoExifInfo.Data_Precision">Data Precision: {{ showPhotoExifInfo.Data_Precision }} </p>  
                    
                  </n-collapse-item>
                  
                  <n-collapse-item title="Secrets" name="se">
                    <p>Token: {{ showPhotoInfo.token }}</p>
                    <p>UUID: {{ showPhotoInfo.photoUuid }}</p>
                  </n-collapse-item>
                  <n-collapse-item title="Tags" name="tag">
                    <n-tag type="success" size="small" round closable @close="removeTag(tagItem.tag_id)" v-for="tagItem in showPhotoTagList">
                      {{ tagItem.tag_name }}
                      
                    </n-tag>

                    <n-divider />
                    <p>Add</p>
                    <!--
                      <n-input-group>
                      <n-input :style="{ width: '100px' }" />
                      <n-button type="primary" ghost style="width: 50px;">
                        <template #icon>
                          <n-icon><Search20Regular/></n-icon>
                        </template>
                      </n-button>
                    </n-input-group>
                    -->
                    <n-select v-model:value="tagVal" :options="tagOptions" v-on:click="getAllTags"/>  
                    <n-button circle style="margin-top: 5px;" @click="addTag">
                      Submit
                    </n-button>
                  </n-collapse-item>
                  <n-collapse-item title="Gallery" name="ga">
                    <n-tag type="success" size="small" round closable @close="removeGa(GaItem.ga_id)" v-for="GaItem in showPhotoGaList">
                      {{ GaItem.ga_name }}
                      
                    </n-tag>
                    <n-divider />
                    <p>Add</p>
                    <!--
                      <n-input-group>
                      <n-input :style="{ width: '100px' }" />
                      <n-button type="primary" ghost style="width: 50px;">
                        <template #icon>
                          <n-icon><Search20Regular/></n-icon>
                        </template>
                      </n-button>
                    </n-input-group>
                    -->

                    <n-select v-model:value="gaVal" :options="gaOptions" v-on:click="getAllGas"/>
                    <n-button circle style="margin-top: 5px;" @click="addGa">
                      Submit
                    </n-button>

                  </n-collapse-item>
                  <n-collapse-item title="Share" name="share">
                    <n-space vertical :size=1>
                      <n-button strong secondary type="default" @click="copyShareAddr" style="width: 10rem;">
                        Copy Share Addr
                      </n-button>
                      <n-popover trigger="click">
                        <template #trigger>
                          <n-button strong secondary type="info" style="margin-top: 10px; width: 10rem">
                            Get QR Code
                          </n-button>
                        </template>
                        <img v-bind:src="baseQR + 'token=' + userToken + '&photoId=' + showPhotoInfo.photoId" style="width:170px;"/>
                      </n-popover>
                    </n-space>
                    
                  </n-collapse-item>

                  <n-collapse-item title="Edit Info" name="ei">
                    <p>Visibility</p>
                    <n-switch v-model:value="showPhotoInfoVisibility" @update:value="changeVisibility" style="margin-left: 8px;"/>
                    <p>Change Name</p>
                    <n-input-group>
                        <n-input type="text" round size="small" placeholder="New Name" v-model:value="newNameVal" @keyup.enter="changeName"  />
                        <n-button type="primary" ghost style="width:5rem" @click="changeName">
                          Update
                        </n-button>
                      </n-input-group>
                  </n-collapse-item>

                </n-collapse>
              </div>
            </div>
            

          </n-card>
        </div>
      </n-modal>

      <!-- ENDOF: This modal is for the photo detail after click on 'view' or open by left click the photo -->

      
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
        :options="PhotoSpaceMenuOptions"
        :show="showPhotoSpaceMenu"
        :on-clickoutside="onClickOutsidePhoto"
        @select="handleSelectPhoto"
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

    <!-- This part is for modal of confirming to delete multiple photos -->
    <n-modal v-model:show="showPhotoMultiDeleteModal">
      <n-card
        style="width: 400px; border-radius: 10px;"
        title="Confirm to Delete"
        :bordered="false"
        size="huge"
        role="dialog"
        aria-modal="true"
      >
        Do you really want to DELETE these photos?
        <p>*All photos will be destroied.</p>

        <n-space style="margin-top: 1rem">
          <n-button type="error" id="bt1" round size="large" style="margin-top: 1rem; width: 100%;" @click="deleteSelectedPhoto">
            Delete
          </n-button>
          <n-button id="bt1" round size="large" style="margin-top: 1rem; width: 100%;" @click="() => {showPhotoMultiDeleteModal = false;}">
            Cancel
          </n-button>
        </n-space>

      </n-card>
    </n-modal>
    <!-- ENDOF: This part is for modal of confirming to delete multiple photos -->

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

export default defineComponent({
  props: {
    nsOrNot: Boolean,
  },
  emits: ["queryNsChildren"],
  expose: ["updateNsId", "updatePhotoList"],
  data() {
    return {
      nsId: -1,
      globalMap: null,
      photoChildren: [],
      nsPath: [],
      nsPathStr: "",
      baseUThumbnail: import.meta.env.VITE_APP_BASE_URL + "/photo/renderThumbnail/" ,
      baseUPhoto: import.meta.env.VITE_APP_BASE_URL + "/photo/render/" ,
      baseQR: import.meta.env.VITE_APP_BASE_URL + "/photo/getQR2?",
      userToken: localStorage.getItem("HRD-Token"),
      showPhoto: ref(false),
      showPhotoInfo: null,
      showPhotoExifInfo: {},
      showPhotoInfoVisibility: false,
      showPhotoTagList: [],
      showPhotoGaList: [],
      tagVal: ref(null),
      gaVal: ref(null),
      tagOptions: [],
      gaOptions: [],
      newNameVal: "",
      baseUVideo: "" ,
      showBlankSpaceMenu: false,
      showPhotoSpaceMenu: false,
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
      PhotoSpaceMenuOptions: [
        {
          label: "View",
          key: "view",
          icon() {
            return h(NIcon, null, {
              default: () => h(Eye20Regular)
            });
          }
        },
        {
          label: "Download",
          key: "download",
          icon() {
            return h(NIcon, null, {
              default: () => h(ArrowDownload20Filled)
            });
          }
        },
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
      
      moveToModalOriginalNsId: -1,
      moveToModalDisableOrgNsId: false,
      
      photoMultiSelectMode: false,
      showPhotoMultiDeleteModal: false,
    }
  },
  setup() {
    return {
      
    };
  },
  methods: {
    updateNsId (newNsId) {
        this.nsId = newNsId;
        this.update();
        if (nsOrNot)
        {
            this.trace(this.nsId);
        }
    },
    updatePhotoList (photoList) {
        this.photoChildren = photoList;
    },
    moveSelectedPhoto() {
      this.moveToModalOriginalNsId = this.nsId;
      this.moveToModalDisableOrgNsId = false;
      this.openMoveToModalMulti();
    },
    deleteSelectedPhoto () {
      this.photoChildren.forEach((photo) => {
        if(photo["checked"])
        {
          axios({
            method: 'post',
            baseURL: '',
            url: import.meta.env.VITE_APP_BASE_URL + "/photo/delete?photoId=" + photo.photoId,
            headers: {
              "HRD-Token": localStorage.getItem("HRD-Token")
            },
            data: {
          }
          }).then((response) => {
            if (response.data.msgCode === 200)
            {
              window.$message.success("Photo deleted!");
              this.queryPhotos();
            }else {
              window.$message.warning("delete photo Response Error!")
            }
          })
          .catch(function (error) { // 请求失败处理
            window.$message.error(error);
          });
        }
      })
      this.photoMultiSelectMode = false;
      this.showPhotoMultiDeleteModal = false;
      this.clearCheckboxPhoto();
      
    },
    selectAllCheckboxPhoto () {
      this.photoChildren.forEach((photo) => {
        photo["checked"] = true;
      })
    },
    clearCheckboxPhoto () {
      this.photoChildren.forEach((photo) => {
        photo["checked"] = false;
      })
    },
    handlePhotoMulti(photo, event)
    {
      if (this.photoMultiSelectMode)
      {
        event.preventDefault();
        event.stopPropagation();
      }else {
        this.photoModalOpen(photo);
      }
    },
    blankSpaceMenu (e) {
        if (nsOrNot)
        {
            e.preventDefault();
            e.stopPropagation();
            this.showBlankSpaceMenu = false;
            nextTick().then(() => {
                this.showBlankSpaceMenu = true;
                this.mouseX = e.clientX;
                this.mouseY = e.clientY;
                });
        }
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
    handleSelectPhoto(key) {
      this.showPhotoSpaceMenu = false;
      if (key === "view")
      {
        this.photoModalOpen(this.showPhotoSpaceMenuPhotoInfo);
      }
      if (key === "download")
      {
        this.showPhotoInfo = this.showPhotoSpaceMenuPhotoInfo;
        this.download();
      }
      if (key === "delete")
      {
        this.deletePhoto();
      }
      if (key === "move_to")
      {
        this.moveToModalOriginalNsId = this.nsId;
        this.moveToModalDisableOrgNsId = false;
        this.openMoveToModal();
      }
    },
    onClickOutsideBlank () {
      this.showBlankSpaceMenu = false;
    },
    onClickOutsidePhoto () {
      this.showPhotoSpaceMenu = false;
    },
    photoSpaceMenu(photo, e) {
      this.showPhotoSpaceMenuPhotoInfo = photo;
      e.preventDefault();
      e.stopPropagation();
      this.showPhotoSpaceMenu = false;
      nextTick().then(() => {
        this.showPhotoSpaceMenu = true;
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
            window.$message.success("Folder created!");
            this.$emit('queryNsChildren');
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
    deletePhoto() {
      axios({
          method: 'post',
          baseURL: '',
          url: import.meta.env.VITE_APP_BASE_URL + "/photo/delete?photoId=" + this.showPhotoSpaceMenuPhotoInfo.photoId,
          headers: {
            "HRD-Token": localStorage.getItem("HRD-Token")
          },
          data: {
        }
        }).then((response) => {
          if (response.data.msgCode === 200)
          {
            this.queryPhotos();
            window.$message.success("Photo deleted!")
          }else {
            window.$message.warning("delete photo Response Error!")
          }
        })
        .catch(function (error) { // 请求失败处理
          window.$message.error(error);
        });
    },
    isphotoOrVideo (format) {
      format = format.toLowerCase();
      let photoFormats = ["jpg", "jpeg", "gif", "png", "webp", "ico", "bmp", "tif", "svg", "psd", "raw", "arw", "pcd", "jfif"]
      let videoFormats = ["mp4", "avi", "wmv", "mpeg", "m4v", "mov", "flv", "asf", "f4v", "rmvb", "vob", "rm"]
      if (photoFormats.includes(format))
      {
        return 1;
      }
      if (videoFormats.includes(format))
      {
        return 2;
      }
      return -1;
    },
    setUpMap() {
      if (this.showPhotoExifInfo.GPS_Longitude && this.showPhotoExifInfo.GPS_Latitude)
      {
        this.globalMap = null;
        this.globalMap = L.map('img_map', {attributionControl: false}).setView([this.showPhotoExifInfo.GPS_Latitude, this.showPhotoExifInfo.GPS_Longitude], 13);
        L.tileLayer('http://a.tile.openstreetmap.org/{z}/{x}/{y}.png', {
            maxZoom: 13,
        }).addTo(this.globalMap);
        var marker = L.marker([this.showPhotoExifInfo.GPS_Latitude, this.showPhotoExifInfo.GPS_Longitude]).addTo(this.globalMap);

        setTimeout(() => {this.globalMap.invalidateSize();}, 100);
      }
      
    },
    getExifInfo() {
      axios({
          method: 'get',
          baseURL: '',
          url: import.meta.env.VITE_APP_BASE_URL + "/photo/getPhotoExif?photoId=" + this.showPhotoInfo.photoId,
          headers: {
            "HRD-Token": localStorage.getItem("HRD-Token")
          },
          data: {
            
        }
        }).then((response) => {
          if (response.data.msgCode === 200)
          {
            this.showPhotoExifInfo = response.data.t;
            this.setUpMap();
            
          }else {
            window.$message.warning("Exif Info Response Error!")
          }
        })
        .catch(function (error) { // 请求失败处理
          window.$message.error(error.$message)
          console.log(error)
        });
    },
    changeName() {
      if (this.newNameVal === "" || this.newNameVal === null)
      {
        window.$message.warning("Image name cannot be empty!")
      }else {
        axios({
          method: 'post',
          baseURL: '',
          url: import.meta.env.VITE_APP_BASE_URL + "/photo/update",
          headers: {
            "HRD-Token": localStorage.getItem("HRD-Token")
          },
          data: {
            "photoId": this.showPhotoInfo.photoId,
            "photoName": this.newNameVal
        }
        }).then((response) => {
          if (response.data.msgCode === 200)
          {
            this.showPhotoInfo.photoName = this.newNameVal;
            this.queryPhotos();
            window.$message.success("Photo name changed!")
          }else {
            window.$message.warning("change name Response Error!")
          }
        })
        .catch(function (error) { // 请求失败处理
          window.$message.error(error);
        });
      }
      
    },
    changeVisibility() {
      this.showPhotoInfoVisibility ? this.showPhotoInfo.visibility = 1 : this.showPhotoInfo.visibility = 0;
      axios({
        method: 'post',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/photo/update",
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
          "photoId": this.showPhotoInfo.photoId,
          "visibility": this.showPhotoInfoVisibility ? 1 : 0
      }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          window.$message.success("visibility changed!")
        }else {
          window.$message.warning("change visibility Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        window.$message.error(error);
      });
    },
    copyShareAddr() {
      axios({
        method: 'get',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/photo/getPath?photoId=" + this.showPhotoInfo.photoId,
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
      }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          navigator.clipboard.writeText(response.data.t);
          window.$message.success("Successful copied to clipboard!")
        }else {
          window.$message.warning("get share addr Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        window.$message.error(error);
      });
    },
    download() {
      axios
          .get(import.meta.env.VITE_APP_BASE_URL + "/photo/download/"+ this.userToken +"?photoId=" + this.showPhotoInfo.photoId, {responseType: "blob"})
          .then(async (response) => {
            var a = document.createElement("a");
            document.body.appendChild(a);
            a.style = "display: none";

            let url = window.URL.createObjectURL(new Blob([response.data]));

            let link = document.createElement("a");
            link.style.display = "none";
            link.href = url;
            link.download = this.showPhotoInfo.photoName + "." + this.showPhotoInfo.format;
            document.body.appendChild(link);
            link.click();
          })
          .catch(function (error) { // 请求失败处理
            // console.log(error);
            window.$message.error(error);
          });
    },
    addTag() {
      axios({
        method: 'post',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/photo/addTags?photoId=" + this.showPhotoInfo.photoId + "&tagIds=" + this.tagVal,
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
      }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          this.tagVal = ref(null);
          this.getTags();
          window.$message.success("added to tag!")
        }else {
          window.$message.error("add tag Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        window.$message.error(error);
      });
    },
    addGa() {
      axios({
        method: 'post',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/photo/addToGallery?photoId=" + this.showPhotoInfo.photoId + "&gaId=" + this.gaVal,
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
      }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          this.gaVal = ref(null);
          this.getGas();
          window.$message.success("added to gallery!")
        }else {
          window.$message.error("add gallery Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        // console.log(error);
        window.$message.error(error);
      });
    },
    getAllTags() {
      axios({
        method: 'get',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/tag/getAll",
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
      }
      }).then((response) => {
        
        if (response.data.msgCode === 200)
        {
          var infoT = response.data.t;
          let alltags = [];
          for(var i = 0; i < infoT.length; i ++)
          {
            var tag = infoT[i];
            alltags.push({
              "label": tag["tagName"], 
              "value": tag["tagId"]
            })
          }
          this.tagOptions = alltags;
        }else {
          window.$message.error("get tags Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        window.$message.error(error);
      });
    },
    getAllGas() {
      axios({
        method: 'get',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/gallery/getAll",
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
      }
      }).then((response) => {
        
        if (response.data.msgCode === 200)
        {
          var infoT = response.data.t;
          let allgas = [];
          for(var i = 0; i < infoT.length; i ++)
          {
            var gas = infoT[i];
            allgas.push({
              "label": gas["gaName"], 
              "value": gas["gaId"]
            })
          }
          this.gaOptions = allgas;
        }else {
          window.$message.error("get galleries Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        window.$message.error(error);
      });
    },
    removeGa(gaId) {
      axios({
        method: 'post',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/photo/deleteFromGallery?photoId=" + this.showPhotoInfo.photoId + "&gaId=" + gaId,
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
      }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          this.getGas();
          window.$message.success("Gallery detached!")
        }else {
          window.$message.error("remove gallery Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        // console.log(error);
        window.$message.error(error);
      });
    },
    removeTag(tagId) {
      axios({
        method: 'post',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/photo/deleteTag?photoId=" + this.showPhotoInfo.photoId + "&tagId=" + tagId,
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
      }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          this.getTags();
          window.$message.success("Tag detached!")
        }else {
          window.$message.error("remove tag Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        // console.log(error);
        window.$message.error(error);
      });
    },
    getTags() {
      axios({
        method: 'get',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/photo/getTagByPhoto?photoId=" + this.showPhotoInfo.photoId,
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
      }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          var infoT = response.data.t;
          this.showPhotoTagList = infoT;
        }else {
          window.$message.error("get tags Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        window.$message.error(error);
      });
    },
    getGas() {
      axios({
        method: 'get',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/photo/getGalleryByPhoto?photoId=" + this.showPhotoInfo.photoId,
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
      }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          var infoT = response.data.t;
          this.showPhotoGaList = infoT;
        }else {
          window.$message.error("get galleries Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        window.$message.error(error);
      });
    },
    photoModalOpen(photoInfo) {
      this.showPhotoInfo = photoInfo;
      this.showPhotoInfoVisibility = this.showPhotoInfo.visibility === 1 ? true : false;
      this.newNameVal = this.showPhotoInfo.photoName;
      this.baseUVideo = import.meta.env.VITE_APP_BASE_URL + "/photo/renderV/" + this.userToken + "?photoId=" + this.showPhotoInfo.photoId
      this.getTags();
      this.getGas();
      this.getExifInfo();
      this.showPhoto = true;
      
      if (!nsOrNot)
      {
        this.trace(photoInfo.nsId);
      }
      
    },
    trace(nsId) {
      axios({
        method: 'get',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/namespace/trace?nsId=" + nsId,
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
        }else {
          window.$message.error("trace Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        window.$message.error(error);
      });
    },
  },
  computed: {

  },
  mounted () {
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
