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
import * as L from 'leaflet';
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
          <n-button round style="width: 12rem" v-show="photoMultiSelectMode" @click="showDetach" type="warning" v-if="detachEnabled">
                Remove From {{ detachStr }}
         </n-button>
          <n-button round style="width: 10rem" v-show="photoMultiSelectMode" @click="() => {showPhotoMultiDeleteModal = true;}" type="error">
              Delete Selected
          </n-button>

          
        </n-space>
        
        <n-space>
        <n-empty v-if="photoChildren.length <= 0" style="padding-left: 30vw; padding-top: 20vh;">
        </n-empty>
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

        <n-space>
          <div style="padding-top: 20vh; " v-show="photoChildren.length < 10">&nbsp;</div>
          <div style="padding-top: 10vh; " v-show="photoChildren.length >= 10">&nbsp;</div>
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
              <img id="showImg" v-show="isphotoOrVideo(showPhotoInfo.format) === 1 && showPhotoIsLoaded" v-bind:src="baseUPhoto + userToken + '?photoId=' + showPhotoInfo.photoId" 
               @click.left.native="() => {showPhoto = false}" @load="onShowPhotoLoad"
              :class="{'longImg': (!showPhotoExifInfo.Image_Height) || (showPhotoExifInfo.Image_Height && Number(showPhotoExifInfo.Image_Height.split(' ')[0]) > Number(showPhotoExifInfo.Image_Width.split(' ')[0])),
               'wideImg': showPhotoExifInfo.Image_Height && Number(showPhotoExifInfo.Image_Height.split(' ')[0]) <= Number(showPhotoExifInfo.Image_Width.split(' ')[0])}"/>
              <n-spin :show="isphotoOrVideo(showPhotoInfo.format) === 1 && !showPhotoIsLoaded">
                <n-skeleton v-show="isphotoOrVideo(showPhotoInfo.format) === 1 && !showPhotoIsLoaded" :sharp="false" style="width:40vw; height:80vh;" />
              </n-spin>
              
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
                    <p v-if="showPhotoExifInfo.File_Size">Size: {{ showPhotoExifInfo.File_Size }} </p>
                    <p>Path: {{ nsPathStr }}</p>
                    <p>Shared: {{ showPhotoInfo.visibility === 0 ? "No" : "Yes" }}</p>
                    <n-space vertical>
                      <n-button strong secondary type="info" @click="seeFullPicture" v-show="isphotoOrVideo(showPhotoInfo.format) === 1">
                      Full Picture
                    </n-button>
                    <n-button strong secondary type="success" @click="download">
                      Download
                    </n-button>
                    </n-space>
                    
                  </n-collapse-item>

                  <n-collapse-item title="Location" name="lo" v-show="showPhotoExifInfo.GPS_Latitude" display-directive="show">
                    <div id="img_map" style="height: 250px; width: 250px;"></div>
                    <n-button strong secondary type="info" style="width: 250px; border-radius: 0px; border-bottom-left-radius: 10px; border-bottom-right-radius: 10px;" @click="openMapInNewTab">
                      Open in Map
                    </n-button>
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
                      <n-button strong secondary type="default" @click="copyShareAddr" >
                        Copy Share Addr
                      </n-button>
                      <n-popover trigger="click">
                        <template #trigger>
                          <n-button strong secondary type="info" style="margin-top: 10px; ">
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
        :options="PhotoSpaceMenuOptions"
        :show="showPhotoSpaceMenu"
        :on-clickoutside="onClickOutsidePhoto"
        @select="handleSelectPhoto"
      />
      <!-- ENDOF: This part is for redesign of right click menu for folder, media, and blank space -->
      <!-- ENDOF: MENU PART -->

    <!-- MODAL PART -->

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

    <!-- This part is for modal of confirming to detach multiple photos -->
    <n-modal v-model:show="showPhotoMultiDetachModal">
        <n-card
            style="width: 400px; border-radius: 10px;"
            title="Confirm to Detach"
            :bordered="false"
            size="huge"
            role="dialog"
            aria-modal="true"
        >
            Do you really want to Detach these photos from this {{ detachStr }}?

            <n-space style="margin-top: 1rem">
            <n-button type="warning" id="bt1" round size="large" style="margin-top: 1rem; width: 100%;" @click="detachPhotos">
                Confirm
            </n-button>
            <n-button id="bt1" round size="large" style="margin-top: 1rem; width: 100%;" @click="() => {showPhotoMultiDetachModal = false;}">
                Cancel
            </n-button>
            </n-space>

        </n-card>
    </n-modal>
    <!-- ENDOF: This part is for modal of confirming to detach multiple photos -->
    

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
  name: 'videoPlayer',
  props: {
    nsOrNot: Boolean,
    detachEnabled: Boolean,
    detachStr: String,
  },
  emits: ["queryPhotos", "doDetach"],
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
      showPhotoIsLoaded: false,
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
      
      showPhotoSpaceMenu: false,
      showPhotoSpaceMenuPhotoInfo: null,
      mouseX: null,
      mouseY: null,
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
      
      moveToModalOriginalNsId: -1,
      moveToModalDisableOrgNsId: false,
      
      photoMultiSelectMode: false,
      showPhotoMultiDeleteModal: false,
      showPhotoMultiDetachModal: false,
    }
  },
  setup() {
    return {
      
    };
  },
  methods: {
    onShowPhotoLoad () {
      return this.showPhotoIsLoaded = true
    },
    seeFullPicture() {
      var url = import.meta.env.VITE_APP_BASE_URL + "/photo/render/" + this.userToken + "?photoId=" + this.showPhotoInfo.photoId;
        window.open(url, '_blank').focus();
    },
    openMapInNewTab() {
        var url = "https://maps.google.com/?q=" + this.showPhotoExifInfo.GPS_Latitude + "," + this.showPhotoExifInfo.GPS_Longitude;
        window.open(url, '_blank').focus();
    },
    detachPhotos() {
        this.$emit('doDetach', this.photoChildren);
        this.showPhotoMultiDetachModal = false;
        this.photoMultiSelectMode = false;
    },
    showDetach () {
        this.showPhotoMultiDetachModal = true;
    },
    updateNsId (newNsId) {
        this.nsId = newNsId;
        if (this.nsOrNot)
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
              this.$emit('queryPhotos');
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
                this.$emit('queryPhotos');

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
            this.$emit('queryPhotos');
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
            this.$emit('queryPhotos');
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

        var customDefault = L.icon({
			iconUrl: 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAABkAAAApCAYAAADAk4LOAAAFgUlEQVR4Aa1XA5BjWRTN2oW17d3YaZtr2962HUzbDNpjszW24mRt28p47v7zq/bXZtrp/lWnXr337j3nPCe85NcypgSFdugCpW5YoDAMRaIMqRi6aKq5E3YqDQO3qAwjVWrD8Ncq/RBpykd8oZUb/kaJutow8r1aP9II0WmLKLIsJyv1w/kqw9Ch2MYdB++12Onxee/QMwvf4/Dk/Lfp/i4nxTXtOoQ4pW5Aj7wpici1A9erdAN2OH64x8OSP9j3Ft3b7aWkTg/Fm91siTra0f9on5sQr9INejH6CUUUpavjFNq1B+Oadhxmnfa8RfEmN8VNAsQhPqF55xHkMzz3jSmChWU6f7/XZKNH+9+hBLOHYozuKQPxyMPUKkrX/K0uWnfFaJGS1QPRtZsOPtr3NsW0uyh6NNCOkU3Yz+bXbT3I8G3xE5EXLXtCXbbqwCO9zPQYPRTZ5vIDXD7U+w7rFDEoUUf7ibHIR4y6bLVPXrz8JVZEql13trxwue/uDivd3fkWRbS6/IA2bID4uk0UpF1N8qLlbBlXs4Ee7HLTfV1j54APvODnSfOWBqtKVvjgLKzF5YdEk5ewRkGlK0i33Eofffc7HT56jD7/6U+qH3Cx7SBLNntH5YIPvODnyfIXZYRVDPqgHtLs5ABHD3YzLuespb7t79FY34DjMwrVrcTuwlT55YMPvOBnRrJ4VXTdNnYug5ucHLBjEpt30701A3Ts+HEa73u6dT3FNWwflY86eMHPk+Yu+i6pzUpRrW7SNDg5JHR4KapmM5Wv2E8Tfcb1HoqqHMHU+uWDD7zg54mz5/2BSnizi9T1Dg4QQXLToGNCkb6tb1NU+QAlGr1++eADrzhn/u8Q2YZhQVlZ5+CAOtqfbhmaUCS1ezNFVm2imDbPmPng5wmz+gwh+oHDce0eUtQ6OGDIyR0uUhUsoO3vfDmmgOezH0mZN59x7MBi++WDL1g/eEiU3avlidO671bkLfwbw5XV2P8Pzo0ydy4t2/0eu33xYSOMOD8hTf4CrBtGMSoXfPLchX+J0ruSePw3LZeK0juPJbYzrhkH0io7B3k164hiGvawhOKMLkrQLyVpZg8rHFW7E2uHOL888IBPlNZ1FPzstSJM694fWr6RwpvcJK60+0HCILTBzZLFNdtAzJaohze60T8qBzyh5ZuOg5e7uwQppofEmf2++DYvmySqGBuKaicF1blQjhuHdvCIMvp8whTTfZzI7RldpwtSzL+F1+wkdZ2TBOW2gIF88PBTzD/gpeREAMEbxnJcaJHNHrpzji0gQCS6hdkEeYt9DF/2qPcEC8RM28Hwmr3sdNyht00byAut2k3gufWNtgtOEOFGUwcXWNDbdNbpgBGxEvKkOQsxivJx33iow0Vw5S6SVTrpVq11ysA2Rp7gTfPfktc6zhtXBBC+adRLshf6sG2RfHPZ5EAc4sVZ83yCN00Fk/4kggu40ZTvIEm5g24qtU4KjBrx/BTTH8ifVASAG7gKrnWxJDcU7x8X6Ecczhm3o6YicvsLXWfh3Ch1W0k8x0nXF+0fFxgt4phz8QvypiwCCFKMqXCnqXExjq10beH+UUA7+nG6mdG/Pu0f3LgFcGrl2s0kNNjpmoJ9o4B29CMO8dMT4Q5ox8uitF6fqsrJOr8qnwNbRzv6hSnG5wP+64C7h9lp30hKNtKdWjtdkbuPA19nJ7Tz3zR/ibgARbhb4AlhavcBebmTHcFl2fvYEnW0ox9xMxKBS8btJ+KiEbq9zA4RthQXDhPa0T9TEe69gWupwc6uBUphquXgf+/FrIjweHQS4/pduMe5ERUMHUd9xv8ZR98CxkS4F2n3EUrUZ10EYNw7BWm9x1GiPssi3GgiGRDKWRYZfXlON+dfNbM+GgIwYdwAAAAASUVORK5CYII=',
			shadowUrl: 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAACkAAAApCAQAAAACach9AAACMUlEQVR4Ae3ShY7jQBAE0Aoz/f9/HTMzhg1zrdKUrJbdx+Kd2nD8VNudfsL/Th///dyQN2TH6f3y/BGpC379rV+S+qqetBOxImNQXL8JCAr2V4iMQXHGNJxeCfZXhSRBcQMfvkOWUdtfzlLgAENmZDcmo2TVmt8OSM2eXxBp3DjHSMFutqS7SbmemzBiR+xpKCNUIRkdkkYxhAkyGoBvyQFEJEefwSmmvBfJuJ6aKqKWnAkvGZOaZXTUgFqYULWNSHUckZuR1HIIimUExutRxwzOLROIG4vKmCKQt364mIlhSyzAf1m9lHZHJZrlAOMMztRRiKimp/rpdJDc9Awry5xTZCte7FHtuS8wJgeYGrex28xNTd086Dik7vUMscQOa8y4DoGtCCSkAKlNwpgNtphjrC6MIHUkR6YWxxs6Sc5xqn222mmCRFzIt8lEdKx+ikCtg91qS2WpwVfBelJCiQJwvzixfI9cxZQWgiSJelKnwBElKYtDOb2MFbhmUigbReQBV0Cg4+qMXSxXSyGUn4UbF8l+7qdSGnTC0XLCmahIgUHLhLOhpVCtw4CzYXvLQWQbJNmxoCsOKAxSgBJno75avolkRw8iIAFcsdc02e9iyCd8tHwmeSSoKTowIgvscSGZUOA7PuCN5b2BX9mQM7S0wYhMNU74zgsPBj3HU7wguAfnxxjFQGBE6pwN+GjME9zHY7zGp8wVxMShYX9NXvEWD3HbwJf4giO4CFIQxXScH1/TM+04kkBiAAAAAElFTkSuQmCC',
            iconSize:     [25, 41], // size of the icon
            shadowSize:   [41, 41], // size of the shadow
            iconAnchor:   [12, 41], // point of the icon which will correspond to marker's location
            shadowAnchor: [12, 41],  // the same for the shadow
        });
        L.Marker.prototype.options.icon = customDefault;
        L.tileLayer('https://a.tile.openstreetmap.org/{z}/{x}/{y}.png', {
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
            this.$emit('queryPhotos');
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
      this.showPhotoIsLoaded = false;
      this.showPhotoInfo = photoInfo;
      this.showPhotoInfoVisibility = this.showPhotoInfo.visibility === 1 ? true : false;
      this.newNameVal = this.showPhotoInfo.photoName;
      this.getTags();
      this.getGas();
      this.getExifInfo();
      this.baseUVideo = import.meta.env.VITE_APP_BASE_VIDEO_URL + "/photo/renderV/" + this.userToken + "?photoId=" + this.showPhotoInfo.photoId
      this.showPhoto = true;
      
      if (!this.nsOrNot)
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
    window.$message = useMessage();
  },
  components: {

  },
  beforeDestroy() {
    if (this.player) {
      this.player.dispose();
    }
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

.longImg {
  object-fit: fill;
  border-radius: 0.375rem;
  max-width:75%;
  height:90vh;
}

.wideImg {
  object-fit: fill;
  border-radius: 0.375rem;
  height: 100%;
  min-width: 30%;
  max-width: 75%;
}

</style>
